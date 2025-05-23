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

func resourceAlicloudMscSubWebhook() *schema.Resource {
	return &schema.Resource{
		Create: resourceAlicloudMscSubWebhookCreate,
		Read:   resourceAlicloudMscSubWebhookRead,
		Update: resourceAlicloudMscSubWebhookUpdate,
		Delete: resourceAlicloudMscSubWebhookDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"server_url": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^https://oapi.dingtalk.com/robot/send\?access_token=[a-zA-Z0-9]+$`), "The serverUrl of the Webhook."),
			},
			"webhook_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile(`^[a-zA-Z]{2,12}$`), "The name must be 2 to 12 characters in length, and can contain uppercase and lowercase letters."),
			},
		},
	}
}

func resourceAlicloudMscSubWebhookCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	request := make(map[string]interface{})
	request["Locale"] = "en"
	request["ServerUrl"] = d.Get("server_url")
	request["WebhookName"] = d.Get("webhook_name")
	request["ClientToken"] = buildClientToken("CreateWebhook")
	var response map[string]interface{}
	var err error
	action := "CreateWebhook"
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		response, err = client.RpcPost("MscOpenSubscription", "2021-07-13", action, nil, request, true)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, "alicloud_msc_sub_webhook", action, AlibabaCloudSdkGoERROR)
	}
	if fmt.Sprint(response["Code"]) != "200" {
		return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
	}

	d.SetId(fmt.Sprint(response["WebhookId"]))

	return resourceAlicloudMscSubWebhookRead(d, meta)
}
func resourceAlicloudMscSubWebhookRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	mscOpenSubscriptionService := MscOpenSubscriptionService{client}
	object, err := mscOpenSubscriptionService.DescribeMscSubWebhook(d.Id())
	if err != nil {
		if NotFoundError(err) {
			log.Printf("[DEBUG] Resource alicloud_msc_sub_webhook mscOpenSubscriptionService.DescribeMscSubWebhook Failed!!! %s", err)
			d.SetId("")
			return nil
		}
		return WrapError(err)
	}
	d.Set("server_url", object["ServerUrl"])
	d.Set("webhook_name", object["WebhookName"])
	return nil
}

func resourceAlicloudMscSubWebhookUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	var err error
	update := false
	request := map[string]interface{}{
		"WebhookId": d.Id(),
	}
	if d.HasChange("server_url") {
		update = true
	}
	request["ServerUrl"] = d.Get("server_url")
	if d.HasChange("webhook_name") {
		update = true
	}
	request["WebhookName"] = d.Get("webhook_name")
	if update {
		request["Locale"] = "en"
		action := "UpdateWebhook"
		request["ClientToken"] = buildClientToken("UpdateWebhook")
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = client.RpcPost("MscOpenSubscription", "2021-07-13", action, nil, request, true)
			if err != nil {
				if NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			addDebug(action, response, request)
			return nil
		})
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
	}
	return resourceAlicloudMscSubWebhookRead(d, meta)
}

func resourceAlicloudMscSubWebhookDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	var err error
	request := map[string]interface{}{
		"WebhookId": d.Id(),
		"Locale":    "en",
	}

	action := "DeleteWebhook"
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		response, err = client.RpcPost("MscOpenSubscription", "2021-07-13", action, nil, request, false)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		addDebug(action, response, request)
		return nil
	})
	if err != nil {
		if IsExpectedErrors(err, []string{"ResourceNotFound"}) || NotFoundError(err) {
			return nil
		}
		return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
	}
	return nil
}
