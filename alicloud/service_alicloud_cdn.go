package alicloud

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PaesslerAG/jsonpath"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
)

type CdnService struct {
	client *connectivity.AliyunClient
}

func (c *CdnService) convertCdnSourcesToString(v []interface{}) (string, error) {
	arrayMaps := make([]interface{}, len(v))
	for i, vv := range v {
		item := vv.(map[string]interface{})
		arrayMaps[i] = map[string]interface{}{
			"content":  item["content"],
			"port":     item["port"],
			"priority": formatInt(item["priority"]),
			"type":     item["type"],
			"weight":   formatInt(item["weight"]),
		}
	}
	maps, err := json.Marshal(arrayMaps)
	if err != nil {
		return "", WrapError(err)
	}
	return string(maps), nil
}

func (c *CdnService) DescribeCdnDomainNew(id string) (*cdn.GetDomainDetailModel, error) {
	model := &cdn.GetDomainDetailModel{}
	request := cdn.CreateDescribeCdnDomainDetailRequest()
	request.RegionId = c.client.RegionId
	request.DomainName = id

	raw, err := c.client.WithCdnClient_new(func(cdnClient *cdn.Client) (interface{}, error) {
		return cdnClient.DescribeCdnDomainDetail(request)
	})

	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDomain.NotFound", "ConfigNotFound"}) {
			return model, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return model, WrapErrorf(err, DefaultErrorMsg, id, request.GetActionName(), AlibabaCloudSdkGoERROR)
	}
	addDebug(request.GetActionName(), raw, request.RpcRequest, request)
	domain, _ := raw.(*cdn.DescribeCdnDomainDetailResponse)
	if domain.GetDomainDetailModel.DomainName != id {
		return model, WrapErrorf(NotFoundErr("cdn_domain", id), NotFoundMsg, ProviderERROR)
	}
	return &domain.GetDomainDetailModel, nil
}

func (s *CdnService) DescribeCdnDomainConfig(id string) (object interface{}, err error) {

	var response map[string]interface{}
	client := s.client
	action := "DescribeCdnDomainConfigs"

	parts := strings.Split(id, ":")
	request := map[string]interface{}{
		"RegionId":      s.client.RegionId,
		"DomainName":    parts[0],
		"FunctionNames": parts[1],
	}

	if len(parts) > 2 {
		request["ConfigId"] = parts[2]
	}

	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("Cdn", "2018-05-10", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		if IsExpectedErrors(err, []string{"InvalidDomain.NotFound"}) {
			return object, WrapErrorf(err, NotFoundMsg, AlibabaCloudSdkGoERROR)
		}
		return object, WrapErrorf(err, DefaultErrorMsg, id, request, AlibabaCloudSdkGoERROR)
	}
	addDebug(action, response, request)

	v, err := jsonpath.Get("$.DomainConfigs.DomainConfig", response)
	if err != nil {
		return object, WrapErrorf(NotFoundErr("cdn_domain_config", id), DefaultErrorMsg, err)
	}

	if len(v.([]interface{})) == 0 {
		return object, WrapErrorf(NotFoundErr("cdn_domain_config", id), ResourceNotfound, response)
	}

	val := v.([]interface{})[0].(map[string]interface{})
	return val, nil
}

func (c *CdnService) WaitForCdnDomain(id string, status Status, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	time.Sleep(DefaultIntervalShort * time.Second)

	for {
		domain, err := c.DescribeCdnDomainNew(id)
		if err != nil {
			if NotFoundError(err) && status == Deleted {
				break
			}
			return WrapError(err)
		}
		if domain.DomainStatus == string(status) {
			break
		}
		time.Sleep(DefaultIntervalShort * time.Second)
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, domain.DomainStatus, status, ProviderERROR)
		}
	}
	return nil
}

func (c *CdnService) DescribeDomainCertificateInfo(id string) (certInfo cdn.CertInfo, err error) {
	request := cdn.CreateDescribeDomainCertificateInfoRequest()
	request.RegionId = c.client.RegionId
	request.DomainName = id
	raw, err := c.client.WithCdnClient_new(func(cdnClient *cdn.Client) (interface{}, error) {
		return cdnClient.DescribeDomainCertificateInfo(request)
	})
	if err != nil {
		return certInfo, WrapErrorf(err, DefaultErrorMsg, id, request.GetActionName(), AlibabaCloudSdkGoERROR)
	}
	addDebug(request.GetActionName(), raw, request.RpcRequest, request)
	response, _ := raw.(*cdn.DescribeDomainCertificateInfoResponse)
	if len(response.CertInfos.CertInfo) <= 0 {
		return certInfo, WrapErrorf(NotFoundErr("DomainCertificateInfo", id), NotFoundMsg, ProviderERROR)
	}
	certInfo = response.CertInfos.CertInfo[0]
	return
}

func (c *CdnService) WaitForServerCertificateNew(id string, serverCertificate string, timeout int) error {
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	for {
		certInfo, err := c.DescribeDomainCertificateInfo(id)
		if err != nil {
			return WrapError(err)
		}
		if strings.TrimSpace(certInfo.ServerCertificate) == strings.TrimSpace(serverCertificate) {
			break
		}
		if time.Now().After(deadline) {
			return WrapErrorf(err, WaitTimeoutMsg, id, GetFunc(1), timeout, strings.TrimSpace(certInfo.ServerCertificate), strings.TrimSpace(serverCertificate), ProviderERROR)
		}
		time.Sleep(DefaultIntervalShort * time.Second)
	}
	return nil
}

func (c *CdnService) DescribeTags(resourceId string, resourceType TagResourceType) (tags []cdn.TagItem, err error) {
	request := cdn.CreateDescribeTagResourcesRequest()
	request.RegionId = c.client.RegionId
	request.ResourceType = string(resourceType)
	request.ResourceId = &[]string{resourceId}
	raw, err := c.client.WithCdnClient_new(func(cdnClient *cdn.Client) (interface{}, error) {
		return cdnClient.DescribeTagResources(request)
	})
	if err != nil {
		err = WrapErrorf(err, DefaultErrorMsg, resourceId, request.GetActionName(), AlibabaCloudSdkGoERROR)
		return
	}
	addDebug(request.GetActionName(), raw, request.RpcRequest, request)
	response, _ := raw.(*cdn.DescribeTagResourcesResponse)
	if len(response.TagResources) < 1 {
		return
	}
	for _, t := range response.TagResources {
		tags = append(tags, t.Tag...)
	}
	return
}

func (c *CdnService) CdnDomainConfigRefreshFunc(id string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		v, err := c.DescribeCdnDomainConfig(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}

		val := v.(map[string]interface{})
		for _, failState := range failStates {
			if fmt.Sprint(val["Status"]) == failState {
				return val, fmt.Sprint(val["Status"]), WrapError(Error(FailedToReachTargetStatus, fmt.Sprint(val["Status"])))
			}
		}
		return val, fmt.Sprint(val["Status"]), nil
	}
}

func (s *CdnService) DescribeCdnRealTimeLogDelivery(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeDomainRealtimeLogDelivery"
	request := map[string]interface{}{
		"Domain": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcGet("Cdn", "2018-05-10", action, request, nil)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		if IsExpectedErrors(err, []string{"Domain.NotFound", "InternalError"}) {
			return object, WrapErrorf(NotFoundErr("CDN:RealTimeLogDelivery", id), NotFoundMsg, ProviderERROR, fmt.Sprint(response["RequestId"]))
		}
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$", response)
	}
	object = v.(map[string]interface{})
	return object, nil
}

func (s *CdnService) CdnRealTimeLogDeliveryStateRefreshFunc(id string, failStates []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		object, err := s.DescribeCdnRealTimeLogDelivery(id)
		if err != nil {
			if NotFoundError(err) {
				// Set this to nil as if we didn't find anything.
				return nil, "", nil
			}
			return nil, "", WrapError(err)
		}

		for _, failState := range failStates {
			if fmt.Sprint(object["Status"]) == failState {
				return object, fmt.Sprint(object["Status"]), WrapError(Error(FailedToReachTargetStatus, fmt.Sprint(object["Status"])))
			}
		}
		return object, fmt.Sprint(object["Status"]), nil
	}
}

func (s *CdnService) DescribeCdnFcTrigger(id string) (object map[string]interface{}, err error) {
	var response map[string]interface{}
	client := s.client
	action := "DescribeFCTrigger"
	request := map[string]interface{}{
		"TriggerARN": id,
	}
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcGet("Cdn", "2018-05-10", action, request, nil)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return object, WrapErrorf(err, DefaultErrorMsg, id, action, AlibabaCloudSdkGoERROR)
	}
	v, err := jsonpath.Get("$.FCTrigger", response)
	if err != nil {
		return object, WrapErrorf(err, FailedGetAttributeMsg, id, "$.FCTrigger", response)
	}
	object = v.(map[string]interface{})
	return object, nil
}
