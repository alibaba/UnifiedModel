# 共享类型

上述 schema 引用的可复用构建块，在此统一记录一次。

## metadata

MetaData 用于定义 UModel 系统中所有元素的元数据。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | `string` | 是 |  | 元素在 UModel 系统中的名称。值不能为空。值格式为小写字母数字字符。 |
| `display_name` | semantic_string (i18n) |  |  | 元素在 UModel 系统中的显示名称。值不能为空。 |
| `description` | semantic_string (i18n) |  |  | 元素在 UModel 系统中的描述。 |
| `short_description` | semantic_string (i18n) |  |  | 元素在 UModel系统中的简短描述。 |
| `domain` | `string` | 是 |  | 域。值格式为'domain.domain'。可以以点连接。 |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | 元素的发布阶段。默认值为 preview。 |
| `icon` | `string` |  |  | 元素在UModel系统中的图标。值格式为'url'。 |
| `uri` | `string` |  |  | 用于表示该元素唯一标识。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于表示该元素的标签。 |
| `properties` | map&lt;string, string&gt; |  |  | 用于表示该元素的附加属性。 |
| `common_schema_info` | object |  |  | 背景：在阿里云可观测业务使用中，内置的UModel数据会作为公共UModel（CommonSchema）默认存在，公共UModel数据以引用方式（CommonSchemaRef）配置在UModel，在查询时动态生成UModel实例并附加该字段作为额外说明。 使用方式：该字段不需要手动配置，仅供查看。若存在该字段，则说明对应的UModel实例只可查看，不支持编辑或删除；若不存在该字段，则可根据业务需求按需编辑。 |
| `common_schema_info.group` | `string` |  |  | CommonSchema的Group信息 |
| `common_schema_info.version` | `string` |  |  | CommonSchema的版本号 |

## schema

用于定义 Schema 的版本信息。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `url` | `string` | 是 |  | Schema 的 URL。Core 部分都为 "umodel.aliyun.com" |
| `version` | `string` | 是 |  | Schema 的版本。命名格式为 "v1.2.3"。 |

## telemetry_data

可观测数据（遥测数据、TelemetryData）的通用表示，从结构上类似于数据库的表，但针对“观测”特点，附加了时间字段。此外增加了若干字段，用于表示该数据集的特性。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `fields` | array&lt;[field_spec](shared-types#field_spec)&gt; |  |  | 可观测数据的字段列表。 |
| `time_field` | `string` |  |  | 可观测数据的时间字段。一般要求时间字段为时间戳类型，支持秒、毫秒、微秒、纳秒。 |
| `display_field` | `string` |  |  | 注意：定位和 `name_fields` 有一定重叠，字段已废弃，请使用 `name_fields` 字段第一个值替代。可观测数据的显示字段。即默认展示的字段。 |
| `name_fields` | array&lt;string&gt; |  |  | 可观测数据的显示字段。建议按照显示重要性配置先后顺序，前端、算法等在各类场景会根据特点自由选择，例如极窄/引用场景下只取name_fields的第一个值。 例如：服务操作实体，建议取值为 ["operation_name", "service_name"]； K8s Pod 实体，建议取值为 ["pod_name", "namespace", "cluster"]。 |
| `hidden_fields` | array&lt;string&gt; |  |  | 可观测数据的隐藏字段。即不用作展示的字段。 |
| `tag_fields` | array&lt;string&gt; |  |  | 可观测数据的标签字段。标签字段默认聚合在一起显示和分析。 |
| `ordered_fields` | array&lt;string&gt; |  |  | 可观测数据的排序字段。即当前数据集的排序字段。 |
| `default_order` | enum: `asc`, `desc` |  |  | 可观测数据的默认排序。默认值为 asc。 |

## link

Link 用于定义两个 Set 之间的关系。是所有 Link 的抽象模块。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `src` | object |  |  | 源 Set 的标识。值不能为空。 |
| `src.domain` | `string` | 是 |  | 源 Set 的域。 |
| `src.kind` | `string` | 是 |  | 源 Set 的类型。值不能为空。 |
| `src.name` | `string` | 是 |  | 源 Set 的名称。值不能为空。 |
| `src.filter` | `string` |  |  | 对于源 Set 的过滤器，用于部分 Link 的场景，例如 EntitySet 中只有"region=cn-beijing"的实体存在此关联。该方式已废弃，请使用 filter_by_entity 替代。 |
| `dest` | object |  |  | 目标 Set 的标识。值不能为空。 |
| `dest.domain` | `string` | 是 |  | 目标 Set 的域。 |
| `dest.kind` | `string` | 是 |  | 目标 Set 的类型。值不能为空。 |
| `dest.name` | `string` | 是 |  | 目标 Set 的名称。值不能为空。 |
| `filter_by_entity` | `string` |  |  | 在部分Entity具备的Link中，用于标识这部分Entity的过滤条件。 |
| `priority` | `integer` |  | `5` | Link 的优先级。值可以为空。默认值为5。 |

## field_spec

字段是日志、指标、追踪、实体等数据的最基本定义，是数据的基本单位。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | `string` | 是 |  | Field 在 UModel 系统中的名称。值不能为空。值格式为小写字母数字字符。 |
| `display_name` | semantic_string (i18n) |  |  | Field 在 UModel 系统中的显示名称。值不能为空。 |
| `description` | semantic_string (i18n) |  |  | Field 在 UModel 系统中的描述。 |
| `short_description` | semantic_string (i18n) |  |  | Field 在 UModel 系统中的简短描述。 |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | 元素的发布阶段。默认值为 preview。 |
| `type` | enum: `string`, `integer`, `float`, `boolean`, `time`, `json_object`, `json_array` | 是 |  | 字段的类型。值必须是以下之一：string、integer、float、boolean、time、json_object、json_array。值不能为空。 |
| `value_mapping` | map&lt;string, string&gt; |  |  | 对于枚举值的映射和解释，为了更好的展示映射值含义，对于 mapping 的 value 字段单独定义，value 包含4个字段："name"、"display_name"、"description"和"short_description"。例如，"status"字段的值是"1"，映射为 ：name 为 "running" ，display_name 为"运行中"，description 为"运行中......"，short_des… |
| `unit` | `string` |  |  | 指标的单位，值格式为字符串。 注意：单位仅用于展示，不会进行格式转换，例如 ms 不会自动根据大小转换为 s。 最佳实践：首先配置 data_format ，在大部分可观测场景下，data_format 即可满足需求，unit 填写为空即可。 若不满足需求，一般建议 data_format 配置通用的 KMB， 并配置 unit 为特定类型。比如温度，可以配置 data_format 为 KMB， 并配置 unit 为 °C。 |
| `data_format` | enum: `KMB`, `milli`, `byte`, `bit`, `byte_ies_sec`, `bit_ies_sec`, `iops`, `reqps` … (47 values) |  | `KMB` | 指标的格式化方式，值格式为字符串。支持的格式化选项： 数值格式化： - KMB: 千、百万、十亿格式化，样例：K,Mil,Bil - milli: 处理为毫秒格式，样例：1,000,000 存储格式化： - byte: 处理为字节格式，样例：B/KB/MB - bit: 处理为位格式，样例：b/Kb/Mb 速率格式化： - byte_ies_sec: 每秒字节（byte per second）格式化，样例：B/s/KB/s/MB/… |
| `analysable` | `bool` |  | `false` | 用于表示字段是否可分析，即作为Group By的列。默认值为false。 |
| `filterable` | `bool` |  | `false` | 用于表示字段是否可过滤，即支持索引过滤。默认值为false。 |
| `orderable` | `bool` |  | `false` | 用于表示字段是否可排序。默认值为false。 |
| `default_order` | enum: `asc`, `desc` |  |  | 用于表示字段的默认排序顺序。值必须是以下之一：asc、desc。默认值为asc。 |
| `pattern` | `string` |  |  | 正则表达式，用于定义字段的值范围（字符串）。 |
| `max_length` | `integer` |  |  | 字符串值的最大长度。仅适用于字符串类型。 |
| `example` | `any` |  |  | 字段值的示例。值可以为空。 |
| `max_value` | `number` |  |  | 字段的最大值。值可以为空。仅适用于整数和浮点类型。 |
| `min_value` | `number` |  |  | 字段的最小值。值可以为空。仅适用于整数和浮点类型。 |
| `default_value` | `any` |  |  | 字段的默认值。值可以为空。 |
| `icon` | `string` |  |  | Field 在 UModel 系统中的图标。值格式为'url'。 |
| `uri` | `string` |  |  | 用于表示该字段唯一标识。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于表示该 Field 的标签。 |
| `properties` | map&lt;string, string&gt; |  |  | 用于表示该 Field 的附加属性。 |

## metric

Metric 用于定义指标信息。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | `string` | 是 |  | 指标的名称。值不能为空。 |
| `display_name` | semantic_string (i18n) |  |  | 指标的显示名称。 |
| `description` | semantic_string (i18n) |  |  | 指标的描述。值格式为语义字符串。 |
| `short_description` | semantic_string (i18n) |  |  | 指标的简短描述。值格式为语义字符串。 |
| `launch_stage` | enum: `preview`, `beta`, `ga`, `deprecated` |  | `preview` | 元素的发布阶段。默认值为 preview。 |
| `unit` | `string` |  |  | 指标的单位，值格式为字符串。 注意：单位仅用于展示，不会进行格式转换，例如 ms 不会自动根据大小转换为 s。 最佳实践：首先配置 data_format ，在大部分可观测场景下，data_format 即可满足需求，unit 填写为空即可。 若不满足需求，一般建议 data_format 配置通用的 KMB， 并配置 unit 为特定类型。比如温度，可以配置 data_format 为 KMB， 并配置 unit 为 °C。 |
| `data_format` | enum: `KMB`, `milli`, `byte`, `bit`, `byte_ies_sec`, `bit_ies_sec`, `iops`, `reqps` … (47 values) |  | `KMB` | 指标的格式化方式，值格式为字符串。支持的格式化选项： 数值格式化： - KMB: 千、百万、十亿格式化，样例：K,Mil,Bil - milli: 处理为毫秒格式，样例：1,000,000 存储格式化： - byte: 处理为字节格式，样例：B/KB/MB - bit: 处理为位格式，样例：b/Kb/Mb 速率格式化： - byte_ies_sec: 每秒字节（byte per second）格式化，样例：B/s/KB/s/MB/… |
| `max_value` | `number` |  |  | 字段的最大值。值可以为空。仅适用于整数和浮点类型。 |
| `min_value` | `number` |  |  | 字段的最小值。值可以为空。仅适用于整数和浮点类型。 |
| `default_value` | `any` |  |  | 字段的默认值。值可以为空。 |
| `icon` | `string` |  |  | 用于表示该指标的图标。 |
| `labels` | array&lt;[field_spec](shared-types#field_spec)&gt; |  |  | 用于表示该指标自身额外的标签。即在 MetricSet 的标签基础上，该指标还具有的额外标签。 |
| `golden_metric` | `boolean` |  | `false` | 是否为黄金指标。 |
| `generator` | `string` |  |  | 指标的生成方式，如果是 PromMode，该字段为 PromQL语句，能够执行并得到一个可直接使用的 Measure 类型指标。 例如：rate(request_count{}[1m])，在一些场景下结合 aggregator 字段，能够计算出聚合维度的指标，例如 sum(rate(request_count{}[1m])) by (label1, label2) 如果是 SQL/SPLMode，该字段为 SQL/SPL语句的计算… |
| `aggregator` | `string` |  |  | 指标的聚合方式。如果指标本身是聚合的，且聚合方式在多种维度组合下均一致，则不需要配置次字段，其他情况需要配置。一些示例场景如下： 1. Prometheus指标和Metric从时间线维度相同 1.1 rate(cpu_usage_seconds_total{}[1m]) 即可计算出单机指标，cpu_usage_seconds_total 代表主机级别的统计值，这时需要配置 Aggregator 为 avg 1.2 avg(rate… |
| `interval_us` | object |  |  | 指标的间隔，值格式为整数或整数数组，单位为微秒。 - 当为单个整数时：表示固定的采集间隔 - 当为整数数组时：表示多个采集间隔，用于支持不同精度的数据采集 |
| `type` | `string` |  | `gauge` | 指标的类型。对于无需二次处理的指标，该字段应固定为 gauge。 |
| `query_mode` | enum: `range`, `instant`, `both` |  |  | 指标期望的查询模式。值必须是以下之一：range, instant, both。 |
| `display_type` | `string` |  |  | 指标的显示类型。当前未使用。 |
| `statistics` | array&lt;string&gt; |  |  | 生成该指标时的统计方式，即该指标有多种生成方式。当前仅在基础云监控场景下使用。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于表示该指标的标签。 |
| `properties` | map&lt;string, string&gt; |  |  | 用于表示该指标的附加属性。 |
| `default_fill_policy` | `string` |  |  | 指标的默认填充策略。 值可以是 null, 0, last, next。 |

## observation

观察配置定义，用于定义现象观察和结论判断。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | `string` | 是 |  | 观察名称，在同一个RunbookSet中必须唯一。 |
| `display_name` | semantic_string (i18n) | 是 |  | 观察显示名称。 |
| `short_description` | semantic_string (i18n) | 是 |  | 观察简短描述。 |
| `description` | semantic_string (i18n) |  |  | 观察详细描述信息。 |
| `phenomenon` | object | 是 |  | 现象定义，描述要观察什么以及如何观察。 |
| `phenomenon.phenomenon_type` | `string` | 是 |  | 现象类型，如query（查询）、action（动作）、dashboard（大盘）、event（事件）等。 |
| `phenomenon.inputs` | array&lt;object&gt; |  |  | 现象相关输入配置，用于指引上层如何选择默认的参数进行填入。包括 Dashboard 变量、Query 输入参数等 |
| `phenomenon.outputs` | array&lt;object&gt; |  |  | 可选，现象相关输出配置，用于指引上层如何解析查询结果。 |
| `phenomenon.properties` | map&lt;string, string&gt; |  |  | 现象相关属性配置，根据 type 类型自由扩展。 |
| `conclusions` | array&lt;object&gt; | 是 |  | 结论配置列表，根据现象观察结果进行判断。 |
| `conclusions.condition_type` | `string` | 是 |  | 结论生效条件类型，根据现象类型，条件执行方式不同。可以是 query/action 执行结果后的 bool 表达式计算，也可以是由 LLM 进行判断的 prompts。 |
| `conclusions.condition` | `string` | 是 |  | 结论生效条件，根据现象类型，条件执行方式不同。 |
| `conclusions.severity` | enum: `info`, `warning`, `error`, `fatal` | 是 |  | 结论严重程度。 |
| `conclusions.display_name` | semantic_string (i18n) | 是 |  | 结论显示名称。 |
| `conclusions.short_description` | semantic_string (i18n) | 是 |  | 结论简短描述。 |
| `conclusions.description` | semantic_string (i18n) |  |  | 结论详细描述。 |
| `conclusions.group` | `string` | 是 |  | 结论分组。在同一个结论分组下，只返回最高等级的结论。 |
| `conclusions.properties` | map&lt;string, string&gt; |  |  | 结论相关属性配置。 |
| `properties` | map&lt;string, string&gt; |  |  | 观察属性配置。 |
| `tags` | map&lt;string, string&gt; |  |  | 观察标签。 |

## value_mapping

ValueMapping 用于定义值的映射关系。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `name` | `string` | 是 |  | 在 UModel 系统中的名称。值不能为空。值格式为小写字母数字字符。 |
| `display_name` | semantic_string (i18n) |  |  | 在 UModel 系统中的显示名称。值不能为空。 |
| `description` | semantic_string (i18n) |  |  | 在 UModel 系统中的描述。 |
| `short_description` | semantic_string (i18n) |  |  | 在 UModel 系统中的简短描述。 |
