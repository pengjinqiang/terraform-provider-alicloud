package alicloud

import (
	"fmt"
	"regexp"
	"time"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

func dataSourceAlicloudGaAcls() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAlicloudGaAclsRead,
		Schema: map[string]*schema.Schema{
			"ids": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
			"name_regex": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.ValidateRegexp,
				ForceNew:     true,
			},
			"names": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
			"acl_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"status": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"active", "configuring", "deleting", "init"}, false),
			},
			"output_file": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"acls": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"acl_entries": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"entry": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"entry_description": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"acl_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"acl_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"address_ip_version": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"enable_details": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func dataSourceAlicloudGaAclsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)

	action := "ListAcls"
	request := make(map[string]interface{})
	if v, ok := d.GetOk("acl_name"); ok {
		request["AclName"] = v
	}
	request["RegionId"] = client.RegionId
	request["MaxResults"] = PageSizeLarge
	var objects []map[string]interface{}
	var aclNameRegex *regexp.Regexp
	if v, ok := d.GetOk("name_regex"); ok {
		r, err := regexp.Compile(v.(string))
		if err != nil {
			return WrapError(err)
		}
		aclNameRegex = r
	}

	idsMap := make(map[string]string)
	if v, ok := d.GetOk("ids"); ok {
		for _, vv := range v.([]interface{}) {
			if vv == nil {
				continue
			}
			idsMap[vv.(string)] = vv.(string)
		}
	}
	status, statusOk := d.GetOk("status")
	var response map[string]interface{}
	var err error
	for {
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(5*time.Minute, func() *resource.RetryError {
			response, err = client.RpcPost("Ga", "2019-11-20", action, nil, request, true)
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
			return WrapErrorf(err, DataDefaultErrorMsg, "alicloud_ga_acls", action, AlibabaCloudSdkGoERROR)
		}
		resp, err := jsonpath.Get("$.Acls", response)
		if err != nil {
			return WrapErrorf(err, FailedGetAttributeMsg, action, "$.Acls", response)
		}
		result, _ := resp.([]interface{})
		for _, v := range result {
			item := v.(map[string]interface{})
			if aclNameRegex != nil && !aclNameRegex.MatchString(fmt.Sprint(item["AclName"])) {
				continue
			}
			if len(idsMap) > 0 {
				if _, ok := idsMap[fmt.Sprint(item["AclId"])]; !ok {
					continue
				}
			}
			if statusOk && status.(string) != "" && status.(string) != item["AclStatus"].(string) {
				continue
			}
			objects = append(objects, item)
		}
		if nextToken, ok := response["NextToken"].(string); ok && nextToken != "" {
			request["NextToken"] = nextToken
		} else {
			break
		}
	}
	ids := make([]string, 0)
	names := make([]interface{}, 0)
	s := make([]map[string]interface{}, 0)
	for _, object := range objects {
		mapping := map[string]interface{}{
			"id":                 fmt.Sprint(object["AclId"]),
			"acl_id":             fmt.Sprint(object["AclId"]),
			"acl_name":           object["AclName"],
			"address_ip_version": object["AddressIPVersion"],
			"status":             object["AclStatus"],
		}
		ids = append(ids, fmt.Sprint(mapping["id"]))
		names = append(names, object["AclName"])
		if detailedEnabled := d.Get("enable_details"); !detailedEnabled.(bool) {
			s = append(s, mapping)
			continue
		}
		id := fmt.Sprint(object["AclId"])
		gaService := GaService{client}
		getResp, err := gaService.DescribeGaAcl(id)
		if err != nil {
			return WrapError(err)
		}
		if v, ok := getResp["AclEntries"].([]interface{}); ok {
			aclEntries := make([]map[string]interface{}, 0)
			for _, val := range v {
				item := val.(map[string]interface{})
				temp := map[string]interface{}{
					"entry":             item["Entry"],
					"entry_description": item["EntryDescription"],
				}

				aclEntries = append(aclEntries, temp)
			}
			mapping["acl_entries"] = aclEntries
		}
		s = append(s, mapping)
	}

	d.SetId(dataResourceIdHash(ids))
	if err := d.Set("ids", ids); err != nil {
		return WrapError(err)
	}

	if err := d.Set("names", names); err != nil {
		return WrapError(err)
	}

	if err := d.Set("acls", s); err != nil {
		return WrapError(err)
	}
	if output, ok := d.GetOk("output_file"); ok && output.(string) != "" {
		writeToFile(output.(string), s)
	}

	return nil
}
