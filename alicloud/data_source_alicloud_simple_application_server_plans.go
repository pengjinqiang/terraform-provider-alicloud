package alicloud

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func dataSourceAlicloudSimpleApplicationServerPlans() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAlicloudSimpleApplicationServerPlansRead,
		Schema: map[string]*schema.Schema{
			"ids": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
			"bandwidth": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"core": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"disk_size": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"flow": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"memory": {
				Type:     schema.TypeFloat,
				Optional: true,
			},
			"platform": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"Linux", "Windows"}, false),
			},
			"output_file": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"plans": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"bandwidth": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"core": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"disk_size": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"flow": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"plan_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"memory": {
							Type:     schema.TypeFloat,
							Computed: true,
						},
						"support_platform": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceAlicloudSimpleApplicationServerPlansRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)

	action := "ListPlans"
	request := make(map[string]interface{})
	request["RegionId"] = client.RegionId
	var objects []map[string]interface{}

	idsMap := make(map[string]string)
	if v, ok := d.GetOk("ids"); ok {
		for _, vv := range v.([]interface{}) {
			if vv == nil {
				continue
			}
			idsMap[vv.(string)] = vv.(string)
		}
	}

	bandwidth, bandwidthOk := d.GetOk("bandwidth")
	core, coreOk := d.GetOk("core")
	diskSize, diskSizeOk := d.GetOk("disk_size")
	memory, memoryOk := d.GetOk("memory")
	flow, flowOk := d.GetOk("flow")

	var response map[string]interface{}
	var err error
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		response, err = client.RpcPost("SWAS-OPEN", "2020-06-01", action, nil, request, true)
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
		return WrapErrorf(err, DataDefaultErrorMsg, "alicloud_simple_application_server_plans", action, AlibabaCloudSdkGoERROR)
	}
	resp, err := jsonpath.Get("$.Plans", response)
	if err != nil {
		return WrapErrorf(err, FailedGetAttributeMsg, action, "$.Plans", response)
	}
	result, _ := resp.([]interface{})
	for _, v := range result {
		item := v.(map[string]interface{})
		if len(idsMap) > 0 {
			if _, ok := idsMap[fmt.Sprint(item["PlanId"])]; !ok {
				continue
			}
		}

		if bandwidthOk && bandwidth.(int) != 0 && bandwidth.(int) != formatInt(item["Bandwidth"]) {
			continue
		}

		if coreOk && core.(int) != 0 && core.(int) != formatInt(item["Core"]) {
			continue
		}

		if diskSizeOk && diskSize.(int) != 0 && diskSize.(int) != formatInt(item["DiskSize"]) {
			continue
		}

		if memoryOk && memory.(float64) != 0 && memory.(float64) != formatFloat64(item["Memory"]) {
			continue
		}

		if flowOk && flow.(int) != 0 && flow.(int) != formatInt(item["Flow"]) {
			continue
		}

		if v, ok := d.GetOk("platform"); ok && v.(string) != "" && !strings.Contains(fmt.Sprint(item["SupportPlatform"]), v.(string)) {
			continue
		}

		objects = append(objects, item)
	}
	ids := make([]string, 0)
	s := make([]map[string]interface{}, 0)
	for _, object := range objects {
		mapping := map[string]interface{}{
			"bandwidth":        formatInt(object["Bandwidth"]),
			"core":             formatInt(object["Core"]),
			"disk_size":        formatInt(object["DiskSize"]),
			"flow":             formatInt(object["Flow"]),
			"id":               fmt.Sprint(object["PlanId"]),
			"plan_id":          fmt.Sprint(object["PlanId"]),
			"memory":           formatFloat64(object["Memory"]),
			"support_platform": fmt.Sprint(object["SupportPlatform"]),
		}
		ids = append(ids, fmt.Sprint(mapping["id"]))
		s = append(s, mapping)
	}

	d.SetId(dataResourceIdHash(ids))
	if err := d.Set("ids", ids); err != nil {
		return WrapError(err)
	}

	if err := d.Set("plans", s); err != nil {
		return WrapError(err)
	}
	if output, ok := d.GetOk("output_file"); ok && output.(string) != "" {
		writeToFile(output.(string), s)
	}

	return nil
}
