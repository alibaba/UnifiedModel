# Shared types

Reusable building blocks referenced by the schemas above. Documented once here.

## metadata

The MetaData module is used to define the meta data of all element in UModel.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | yes |  | The name of the Element in UModel System. The value cannot be empty.The value format is lowercase alphanumeric characters. |
| `display_name` | semantic_string (i18n) |  |  | The display name of the Element in UModel System. The value cannot be empty. |
| `description` | semantic_string (i18n) |  |  | The description of the Element in UModel System. |
| `short_description` | semantic_string (i18n) |  |  | The short description of the Element in UModel System. |
| `domain` | `string` | yes |  | The domain of the Field. The value format is 'domain.domain'. it can be connected in the form of a dot. |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | The launch stage of the Element. The default value is preview. |
| `icon` | `string` |  |  | The icon of the Element in UModel System. The value format is 'url'. |
| `uri` | `string` |  |  | This field is used to represent the unique identifier of the Element. |
| `tags` | map&lt;string, string&gt; |  |  | This field is used to represent the tags of the Element. |
| `properties` | map&lt;string, string&gt; |  |  | This field is used to represent the additional properties of the Element. |
| `common_schema_info` | object |  |  | Background: In the use of Alibaba Cloud Observability business, the built-in UModel data will be defaulted as a common UModel (CommonSchema), and the common UModel data will be configured as a reference (CommonSchemaR… |
| `common_schema_info.group` | `string` |  |  | The group of the common schema |
| `common_schema_info.version` | `string` |  |  | The version of the common schema |

## schema

The Schema module is used to define the version information of the Schema.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `url` | `string` | yes |  | The url of the schema. The Core part is "umodel.aliyun.com" |
| `version` | `string` | yes |  | The version of the schema. The format is "v1.2.3". |

## telemetry_data

The general representation of observable data, which is similar to the structure of a database table, but with the addition of time fields for the "observation" characteristics. Additionally, several fields are added…

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `fields` | array&lt;[field_spec](shared-types#field_spec)&gt; |  |  | The fields of the TelemetryData. |
| `time_field` | `string` |  |  | The time field of the TelemetryData. It is generally required to be a timestamp type, supporting seconds, milliseconds, microseconds, and nanoseconds. |
| `display_field` | `string` |  |  | Note: This field is deprecated. Please use the `name_fields` field instead. The display field of the TelemetryData. It is the default displayed field. |
| `name_fields` | array&lt;string&gt; |  |  | The name fields of the TelemetryData. It is recommended to configure the display fields in order of importance. Front-end, algorithms, etc. will freely choose according to the characteristics in various scenarios. For… |
| `hidden_fields` | array&lt;string&gt; |  |  | The hidden fields of the TelemetryData. |
| `tag_fields` | array&lt;string&gt; |  |  | The tag fields of the TelemetryData. The tag fields are aggregated by default for display and analysis. |
| `ordered_fields` | array&lt;string&gt; |  |  | The ordered fields of the TelemetryData. It is the ordered field of the current data set. |
| `default_order` | enum: `asc`, `desc` |  |  | The default order of the TelemetryData. The default value is asc. |

## link

The Link module is used to define the relationship between two entity set, data set, storage set and metric set. The Link module is the abstract module of all link module.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `src` | object |  |  | The identify of source Set. The value cannot be empty. |
| `src.domain` | `string` | yes |  | The domain of the source Set. |
| `src.kind` | `string` | yes |  | The type of the source Set. The value cannot be empty. |
| `src.name` | `string` | yes |  | The name of the source Set. The value cannot be empty. |
| `src.filter` | `string` |  |  | The filter of the source Set. It is used for some Link scenarios, such as only entities with "region=cn-beijing" in the EntitySet have this association. This method is deprecated, please use filter_by_entity instead. |
| `dest` | object |  |  | The identify of the destination Set. The value cannot be empty. |
| `dest.domain` | `string` | yes |  | The domain of the destination Set. |
| `dest.kind` | `string` | yes |  | The type of the destination Set. The value cannot be empty. |
| `dest.name` | `string` | yes |  | The name of the destination Set. The value cannot be empty. |
| `filter_by_entity` | `string` |  |  | In the Link that is only for some entities, used to identify the filtering conditions for these entities. |
| `priority` | `integer` |  | `5` | The priority of the Link. The value can be empty. The default value is 5. |

## field_spec

The most basic definition of a field, which is the fundamental unit of data for logs, metrics, traces, entities, etc.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | yes |  | The name of the Field in UModel System. The value cannot be empty.The value format is lowercase alphanumeric characters. |
| `display_name` | semantic_string (i18n) |  |  | The display name of the Element in UModel System. The value cannot be empty. |
| `description` | semantic_string (i18n) |  |  | The description of the Element in UModel System. |
| `short_description` | semantic_string (i18n) |  |  | The short description of the Element in UModel System. |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | The launch stage of the Element. The default value is preview. |
| `type` | enum: `string`, `integer`, `float`, `boolean`, `time`, `json_object`, `json_array` | yes |  | The type of the Field. The value must be one of the following: string, integer, float, boolean, time, json_object, json_array. The value cannot be empty. |
| `value_mapping` | map&lt;string, string&gt; |  |  | For the mapping and interpretation of enumeration values, in order to better display the meaning of the mapping value, the value field of the mapping is defined separately. The value contains 4 fields: "name", "displa… |
| `unit` | `string` |  |  | The unit of the Metric. The value format is a string. Note: The unit is only used for display and will not be converted, such as ms will not be automatically converted to s. Best practice: First configure data_format,… |
| `data_format` | enum: `KMB`, `milli`, `byte`, `bit`, `byte_ies_sec`, `bit_ies_sec`, `iops`, `reqps` … (47 values) |  | `KMB` | The formatting method for metrics, value format is string. Supported formatting options: Numeric formatting: - KMB: Thousands, millions, billions formatting, example: K,Mil,Bil - milli: Process as millisecond format,… |
| `analysable` | `bool` |  | `false` | Used to denote whether the field is analyzable, i.e., serving as a column for Group By. default value is false. |
| `filterable` | `bool` |  | `false` | Used to indicate whether this field can support filtering, i.e., it supports indexed filtering. default value is false. |
| `orderable` | `bool` |  | `false` | Used to indicate whether the field is sortable. default value is false. |
| `default_order` | enum: `asc`, `desc` |  |  | Used to indicate the default sorting order of the field. The value must be one of the following: asc, desc. default value is asc. |
| `pattern` | `string` |  |  | Regular expression, used to define the range of values for this field (String). |
| `max_length` | `integer` |  |  | The max length of string value. Only valid for string type. |
| `example` | `any` |  |  | examples of the field value. The value can be empty. |
| `max_value` | `number` |  |  | The maximum value of the field. The value can be empty. Only valid for integer and float types. |
| `min_value` | `number` |  |  | The minimum value of the field. The value can be empty. Only valid for integer and float types. |
| `default_value` | `any` |  |  | The default value of the field. The value can be empty. |
| `icon` | `string` |  |  | The icon of the Element in UModel System. The value format is 'url'. |
| `uri` | `string` |  |  | This field is used to represent the unique identifier of the field. |
| `tags` | map&lt;string, string&gt; |  |  | This field is used to represent the tags of the Field. |
| `properties` | map&lt;string, string&gt; |  |  | This field is used to represent the additional properties of the Field. |

## metric

The Metric module is used to define the metric information.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | yes |  | The name of the Metric. The value cannot be empty. |
| `display_name` | semantic_string (i18n) |  |  | The display name of the Metric. The value cannot be empty. |
| `description` | semantic_string (i18n) |  |  | The description of the Metric. |
| `short_description` | semantic_string (i18n) |  |  | The short description of the Metric. |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | The launch stage of the Element. The default value is preview. |
| `unit` | `string` |  |  | The unit of the Metric. The value format is a string. Note: The unit is only used for display and will not be converted, such as ms will not be automatically converted to s. Best practice: First configure data_format,… |
| `data_format` | enum: `KMB`, `milli`, `byte`, `bit`, `byte_ies_sec`, `bit_ies_sec`, `iops`, `reqps` … (47 values) |  | `KMB` | The formatting method for metrics, value format is string. Supported formatting options: Numeric formatting: - KMB: Thousands, millions, billions formatting, example: K,Mil,Bil - milli: Process as millisecond format,… |
| `max_value` | `number` |  |  | The maximum value of the field. The value can be empty. Only valid for integer and float types. |
| `min_value` | `number` |  |  | The minimum value of the field. The value can be empty. Only valid for integer and float types. |
| `default_value` | `any` |  |  | The default value of the field. The value can be empty. |
| `icon` | `string` |  |  | This field is used to represent the icon of the Metric. |
| `labels` | array&lt;[field_spec](shared-types#field_spec)&gt; |  |  | This field is used to represent the labels of the Metric. It is the additional labels of the Metric on top of the labels in the MetricSet. |
| `golden_metric` | `boolean` |  | `false` | Whether the metric is a golden metric. |
| `generator` | `string` |  |  | This field represents the generator of the Metric. If the mode is PromMode, this field is a PromQL statement that can be executed to get a Measure type metric directly. For example: rate(request_count{}[1m]). If the m… |
| `aggregator` | `string` |  |  | The aggregation method of the metric. If the metric is already aggregated, and the aggregation method is consistent in multiple dimension combinations, this field does not need to be configured, otherwise it needs to… |
| `interval_us` | object |  |  | The interval of the metric, value format is integer or array of integers, in microseconds. - When single integer: represents a fixed collection interval - When array of integers: represents multiple collection interva… |
| `type` | `string` |  | `gauge` | This field indicates the metric type. For metrics that do not need to be processed again, this field should be fixed to gauge. |
| `query_mode` | enum: `range`, `instant`, `both` |  |  | This metric query mode. The value must be one of the following: range, instant, both. |
| `display_type` | `string` |  |  | This metric display type. |
| `statistics` | array&lt;string&gt; |  |  | The statistics method when generating this metric, that is, the metric has multiple generation methods. It is currently only used in the cloud monitor service scenario. |
| `tags` | map&lt;string, string&gt; |  |  | This field is used to represent the tags of the Metric. |
| `properties` | map&lt;string, string&gt; |  |  | This field is used to represent the additional properties of the Metric. |
| `default_fill_policy` | `string` |  |  | The default fill policy of the Metric. The value can be null, 0, last, next. |

## observation

Observation configuration definition for phenomenon observation and conclusion judgment.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | yes |  | Observation name, must be unique within the same RunbookSet. |
| `display_name` | semantic_string (i18n) | yes |  | Observation display name. |
| `short_description` | semantic_string (i18n) | yes |  | Observation short description. |
| `description` | semantic_string (i18n) |  |  | Observation detailed description. |
| `phenomenon` | object | yes |  | Phenomenon definition, describing what to observe and how. |
| `phenomenon.phenomenon_type` | `string` | yes |  | Phenomenon type such as query, action, dashboard, event. |
| `phenomenon.inputs` | array&lt;object&gt; |  |  | Phenomenon related input configurations, used to guide the upper layer to select default parameters for filling in. Including Dashboard variables, Query input parameters, etc. |
| `phenomenon.outputs` | array&lt;object&gt; |  |  | Optional, phenomenon related output configurations, used to guide the upper layer to parse the query results. |
| `phenomenon.properties` | map&lt;string, string&gt; |  |  | Phenomenon related properties, freely extensible based on type. |
| `conclusions` | array&lt;object&gt; | yes |  | List of conclusion configurations based on phenomenon observation results. |
| `conclusions.condition_type` | `string` | yes |  | Condition type for conclusion activation. Can be bool expression calculated after query/action execution, or prompts for LLM to judge. |
| `conclusions.condition` | `string` | yes |  | Condition for conclusion activation. |
| `conclusions.severity` | enum: `info`, `warning`, `error`, `fatal` | yes |  | Conclusion severity level. |
| `conclusions.display_name` | semantic_string (i18n) | yes |  | Conclusion display name. |
| `conclusions.short_description` | semantic_string (i18n) | yes |  | Conclusion short description. |
| `conclusions.description` | semantic_string (i18n) |  |  | Conclusion detailed description. |
| `conclusions.group` | `string` | yes |  | Conclusion group. In the same conclusion group, only the highest level of conclusion is returned. |
| `conclusions.properties` | map&lt;string, string&gt; |  |  | Conclusion related properties. |
| `properties` | map&lt;string, string&gt; |  |  | Observation property configurations. |
| `tags` | map&lt;string, string&gt; |  |  | Observation tags. |

## value_mapping

The ValueMapping module is used to define the value mapping relationship.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | `string` | yes |  | The name of the Element in UModel System. The value cannot be empty.The value format is lowercase alphanumeric characters. |
| `display_name` | semantic_string (i18n) |  |  | The display name of the Element in UModel System. The value cannot be empty. |
| `description` | semantic_string (i18n) |  |  | The description of the Element in UModel System. |
| `short_description` | semantic_string (i18n) |  |  | The short description of the Element in UModel System. |
