# UModel Schema 概述

UModel Schema，作为UModel实体元数据的语法体系，封装了一套全面且精确的定义语法，旨在赋能开发者构造任意复杂的UModel模型。

## 📋 目录结构

```
schemas/
├── README.md                   
├── manifest.yaml               # Schema清单文件，定义所有支持的模型版本
├── base.yaml                   # 基础Schema元数据定义规范
├── includes/                   # 通用Schema元素定义
│   ├── field_spec.schema.yaml      # 字段定义
│   ├── metric.schema.yaml          # 指标定义
│   ├── link.schema.yaml            # Link定义
│   ├── metadata.schema.yaml        # 元数据定义
│   ├── value_mapping.schema.yaml   # 值映射定义
│   ├── schema.schema.yaml          # Schema基础定义
│   └── telemetry_data.schema.yaml  # 遥测数据定义
└── core/                       # 核心模型Schema定义
    ├── dataset/                    # 数据集相关Schema
    │   ├── entity_set.schema.yaml      # 实体集定义
    │   ├── metric_set.schema.yaml      # 指标集定义
    │   ├── log_set.schema.yaml         # 日志集定义
    │   ├── trace_set.schema.yaml       # 追踪集定义
    │   └── event_set.schema.yaml       # 事件集定义
    ├── link/                       # 关联关系Schema
    │   ├── data_link.schema.yaml       # 数据链接定义
    │   ├── entity_set_link.schema.yaml # 实体链接定义
    │   └── storage_link.schema.yaml    # 存储链接定义
    └── storage/                    # 存储相关Schema
        ├── sls_logstore.schema.yaml     # SLS日志存储定义
        ├── sls_metricstore.schema.yaml  # SLS指标存储定义
        ├── sls_entitystore.schema.yaml  # SLS实体存储定义
        ├── aliyun_prometheus.schema.yaml # 阿里云Prometheus存储定义
        ├── prometheus.schema.yaml        # 开源Prometheus存储定义
        ├── mysql.schema.yaml             # MySQL存储定义
        └── elasticsearch.schema.yaml     # Elasticsearch存储定义
```

## 🎯 核心概念

### Schema层次结构

UModel Schema采用分层设计，包含以下几个层次：

1. **基础层 (base.yaml)**: 定义Schema的元数据规范和基础类型
2. **组件层 (includes/)**: 定义可复用的通用组件，如字段、指标、链接等
3. **核心层 (core/)**: 定义具体的业务模型，包括数据集、关联关系和存储

### 主要模型类型

#### 数据集模型 (Dataset)
- **EntitySet**: 实体集合，定义一类实体资源的结构和属性
- **MetricSet**: 指标集合，包含多个相关指标的定义
- **LogSet**: 日志集合，定义日志数据的结构
- **TraceSet**: 追踪集合，定义分布式追踪数据结构
- **EventSet**: 事件集合，定义事件数据的结构

#### 关联关系模型 (Link)
- **DataLink**: 数据之间的关联关系
- **EntitySetLink**: 实体集之间的关联关系
- **StorageLink**: 存储之间的关联关系

#### 存储模型 (Storage)
- **SLS LogStore**: 阿里云SLS日志存储
- **SLS MetricStore**: 阿里云SLS指标存储
- **SLS EntityStore**: 阿里云SLS实体存储
- **Aliyun Prometheus**: 阿里云Prometheus存储
- **Prometheus**: 开源 Prometheus 或 Prometheus 兼容存储
- **MySQL**: MySQL 数据库存储
- **Elasticsearch**: Elasticsearch 索引存储

## 🔧 Schema语法特性

### 多语言支持
所有Schema定义都支持中英文双语描述，使用`semantic_string`类型：

```yaml
description:
  zh_cn: 中文描述
  en_us: English description
```

### 类型系统
支持丰富的数据类型：
- **基础类型**: string, integer, float, boolean, time
- **复合类型**: object, array, map
- **特殊类型**: json_object, json_array, semantic_string

### 约束系统
提供完整的约束定义：
- **必填约束**: `required: true`
- **格式约束**: `pattern: "^[a-zA-Z].*"`
- **长度约束**: `min_len`, `max_len`
- **枚举约束**: `enum: [value1, value2]`

### 继承机制
支持类型继承和复用：
```yaml
extends:
  - field_spec:v1
  - metadata:v1
```

## 📖 使用指南

### 1. 查看支持的模型
查看`manifest.yaml`文件了解当前支持的所有模型和版本：

```yaml
models:
  - entity_set:v1.0.0
  - metric_set:v1.0.0
  - log_set:v1.0.0
  # ... 更多模型
```

### 2. 理解基础规范
阅读`base.yaml`了解Schema定义的基础规范和元数据结构。

### 3. 使用通用组件
在`includes/`目录中查找可复用的组件定义，如字段规格、指标定义等。

### 4. 定义具体模型
根据业务需求，参考`core/`目录中的模型定义创建自己的Schema。

## 🌟 最佳实践

### Schema设计原则
1. **模块化**: 将通用组件抽取到`includes/`目录
2. **版本化**: 为每个Schema定义明确的版本号
3. **文档化**: 提供详细的中英文描述
4. **约束化**: 定义适当的约束条件确保数据质量

### 命名规范
- Schema文件名使用小写字母和下划线：`entity_set.schema.yaml`
- 字段名使用小写字母和下划线：`display_name`
- 版本号使用语义化版本：`v1.0.0`

### 扩展指南
1. 新增模型时，先在`manifest.yaml`中注册
2. 定义通用组件时，放置在`includes/`目录
3. 业务特定模型放置在`core/`的相应子目录
4. 确保所有Schema都有完整的约束定义

## 🔗 相关资源

- [UModel项目主页](../README.md)

## 📝 版本历史

- **v0.1.0**: 初始版本，包含基础模型定义
- 更多版本信息请查看`manifest.yaml`

---

> 💡 **提示**: 如需了解具体模型的详细定义，请查看对应的`.schema.yaml`文件。每个文件都包含完整的字段定义、约束条件和使用示例。
> 💡 **提示**: 完整展开后的模型定义在 `expanded_schemas` 目录。


