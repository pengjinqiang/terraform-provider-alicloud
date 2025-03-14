package alicloud

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

func resourceAlicloudCddcDedicatedHostAccount() *schema.Resource {
	return &schema.Resource{
		Create: resourceAlicloudCddcDedicatedHostAccountCreate,
		Read:   resourceAlicloudCddcDedicatedHostAccountRead,
		Update: resourceAlicloudCddcDedicatedHostAccountUpdate,
		Delete: resourceAlicloudCddcDedicatedHostAccountDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"account_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^[a-z][a-z0-9_]{0,14}[a-z0-9]$`), "The account name must be 2 to 16 characters in length, and can contain lower case letters, digits, and underscore(_)."),
			},
			"account_password": {
				Type:         schema.TypeString,
				Required:     true,
				Sensitive:    true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^[\w!@#$%^&*()_+=]{6,32}$`), "The account password must be 6 to 32 characters in length, and can contain letters, digits, and special characters（!@#$%^&*()_+-=)."),
			},
			"account_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"Admin", "Normal"}, false),
			},
			"dedicated_host_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceAlicloudCddcDedicatedHostAccountCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	action := "CreateDedicatedHostAccount"
	request := make(map[string]interface{})
	var err error
	request["AccountName"] = d.Get("account_name")
	request["AccountPassword"] = d.Get("account_password")
	if v, ok := d.GetOk("account_type"); ok {
		request["AccountType"] = v
	}
	request["DedicatedHostId"] = d.Get("dedicated_host_id")
	request["RegionId"] = client.RegionId
	request["ClientToken"] = buildClientToken("CreateDedicatedHostAccount")
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		response, err = client.RpcPost("cddc", "2020-03-20", action, nil, request, true)
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
		return WrapErrorf(err, DefaultErrorMsg, "alicloud_cddc_dedicated_host_account", action, AlibabaCloudSdkGoERROR)
	}

	d.SetId(fmt.Sprint(request["DedicatedHostId"], ":", request["AccountName"]))
	cddcService := CddcService{client}
	stateConf := BuildStateConf([]string{}, []string{fmt.Sprint(request["AccountName"])}, d.Timeout(schema.TimeoutCreate), 5*time.Second, cddcService.CddcDedicatedHostAccountStateRefreshFunc(d, []string{}))
	if _, err := stateConf.WaitForState(); err != nil {
		return WrapErrorf(err, IdMsg, d.Id())
	}

	return resourceAlicloudCddcDedicatedHostAccountRead(d, meta)
}
func resourceAlicloudCddcDedicatedHostAccountRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	cddcService := CddcService{client}
	object, err := cddcService.DescribeCddcDedicatedHostAccount(d.Id())
	if err != nil {
		if NotFoundError(err) {
			log.Printf("[DEBUG] Resource alicloud_cddc_dedicated_host_account cddcService.DescribeCddcDedicatedHostAccount Failed!!! %s", err)
			d.SetId("")
			return nil
		}
		return WrapError(err)
	}
	d.Set("account_name", object["AccountName"])
	d.Set("account_type", object["AccountType"])
	d.Set("dedicated_host_id", object["DedicatedHostId"])
	return nil
}
func resourceAlicloudCddcDedicatedHostAccountUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	parts, err := ParseResourceId(d.Id(), 2)
	if err != nil {
		return WrapError(err)
	}
	request := map[string]interface{}{
		"AccountName":     parts[1],
		"DedicatedHostId": parts[0],
	}
	if d.HasChange("account_password") {
		request["AccountPassword"] = d.Get("account_password")
	}
	request["RegionId"] = client.RegionId
	action := "ModifyDedicatedHostAccount"
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
		response, err = client.RpcPost("cddc", "2020-03-20", action, nil, request, false)
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
		return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
	}
	return resourceAlicloudCddcDedicatedHostAccountRead(d, meta)
}
func resourceAlicloudCddcDedicatedHostAccountDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	parts, err := ParseResourceId(d.Id(), 2)
	if err != nil {
		return WrapError(err)
	}
	action := "DeleteDedicatedHostAccount"
	var response map[string]interface{}
	request := map[string]interface{}{
		"AccountName":     parts[1],
		"DedicatedHostId": parts[0],
	}

	request["RegionId"] = client.RegionId
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		response, err = client.RpcPost("cddc", "2020-03-20", action, nil, request, false)
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
		return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
	}
	// wait for complete async operation
	time.Sleep(2 * time.Minute)
	return nil
}
