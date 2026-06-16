#!/usr/bin/env python3
"""
UModel 配置文件验证器

这个脚本用于验证配置YAML文件是否符合expanded_schemas中定义的schema规范。
复用schema_validator.py中的验证逻辑，基于base.yaml的元数据规则进行验证。

功能：
1. 根据配置文件中的kind字段自动选择对应的schema
2. 验证配置文件的结构和数据类型
3. 检查必填字段
4. 验证字段值的约束条件（枚举、正则、长度等）
5. 生成详细的验证报告
6. 批量验证目录下所有YAML文件（递归）

使用方法：
    # 验证单个文件
    python umodel_validator.py <config_file>
    python umodel_validator.py examples/dataset/metricset/sls.front.metric.yaml
    
    # 批量验证目录（递归）
    python umodel_validator.py --batch <directory>
    python umodel_validator.py --batch examples/
    
    # 批量验证目录（非递归）
    python umodel_validator.py --batch <directory> --no-recursive
    
    # 启用控制台日志输出
    python umodel_validator.py --console-log <config_file>
"""

import os
import sys
import yaml
import re
import logging
import argparse
from pathlib import Path
from typing import Dict, Any, List, Optional, Union, Tuple

# 导入schema_validator中的基础验证类
from schema_validator import SchemaValidator


class ConfigValidator(SchemaValidator):
    """配置文件验证器，继承自SchemaValidator以复用验证逻辑"""
    
    def __init__(self, expanded_schemas_dir: str = "expanded_schemas", base_schema_path: str = "schemas/base.yaml", console_log: bool = False):
        # 调用父类初始化
        super().__init__(expanded_schemas_dir, base_schema_path)
        
        # 配置日志
        self.console_log = console_log
        self.logger = logging.getLogger('umodel_validator')
        
        # 加载所有展开后的schema定义
        self.schemas: Dict[str, Dict[str, Any]] = {}
        self._load_expanded_schemas()
    
    def log(self, message: str, level: str = "info"):
        """统一的日志输出方法"""
        if self.console_log:
            # 当console_log=True时才输出
            print(message)
    
    def _load_expanded_schemas(self):
        """加载所有展开后的schema定义"""
        if not self.expanded_schemas_dir.exists():
            raise FileNotFoundError(f"展开后的schema目录不存在: {self.expanded_schemas_dir}")
        
        for schema_file in self.expanded_schemas_dir.glob("*.expanded.yaml"):
            try:
                with open(schema_file, 'r', encoding='utf-8') as f:
                    schema_content = yaml.safe_load(f)
                
                if schema_content and 'name' in schema_content:
                    schema_name = schema_content['name']
                    self.schemas[schema_name] = schema_content
                    self.log(f"✅ 已加载schema: {schema_name}")
            except Exception as e:
                self.log(f"❌ 加载schema失败 {schema_file}: {e}", "error")
        
        self.log(f"📊 共加载 {len(self.schemas)} 个schema定义")
    
    def find_yaml_files(self, directory: str, recursive: bool = True) -> List[Path]:
        """查找目录下的所有YAML文件"""
        directory_path = Path(directory)
        if not directory_path.exists():
            raise FileNotFoundError(f"目录不存在: {directory}")
        
        if not directory_path.is_dir():
            raise ValueError(f"路径不是目录: {directory}")
        
        yaml_files = []
        pattern = "**/*.yaml" if recursive else "*.yaml"
        
        # 查找.yaml文件
        yaml_files.extend(directory_path.glob(pattern))
        
        # 查找.yml文件
        pattern = "**/*.yml" if recursive else "*.yml"
        yaml_files.extend(directory_path.glob(pattern))
        
        # 跳过非模型资产目录(与 Go importer 的 shouldSkipDir 一致:部署脚本等不是模型文件)
        skip_dirs = {"node_modules", "vendor", "target", "dist", "build", "sample-data", "deploy"}
        yaml_files = [f for f in yaml_files if not (skip_dirs & set(f.parts))]

        # 排序并去重
        yaml_files = sorted(list(set(yaml_files)))
        
        self.log(f"🔍 在目录 '{directory}' 中找到 {len(yaml_files)} 个YAML文件")
        if recursive:
            self.log("   (递归搜索)")
        else:
            self.log("   (非递归搜索)")
        
        return yaml_files
    
    def validate_batch(self, directory: str, recursive: bool = True) -> Dict[str, Any]:
        """批量验证目录下的所有YAML文件"""
        print(f"🚀 开始批量验证目录: {directory}")
        print("=" * 60)
        
        try:
            yaml_files = self.find_yaml_files(directory, recursive)
            
            if not yaml_files:
                return {
                    "total_files": 0,
                    "valid_files": 0,
                    "invalid_files": 0,
                    "skipped_files": 0,
                    "results": {},
                    "summary": "未找到YAML文件"
                }
            
            results = {}
            valid_count = 0
            invalid_count = 0
            skipped_count = 0
            
            for i, yaml_file in enumerate(yaml_files, 1):
                print(f"\n📄 [{i}/{len(yaml_files)}] 验证文件: {yaml_file}")
                print("-" * 40)
                
                try:
                    result = self.validate_config(str(yaml_file))
                    results[str(yaml_file)] = result
                    
                    if result["valid"]:
                        valid_count += 1
                        print("✅ 验证通过")
                    else:
                        invalid_count += 1
                        print(f"❌ 验证失败 ({len(result['errors'])} 个错误)")
                        
                except Exception as e:
                    skipped_count += 1
                    error_result = {
                        "valid": False,
                        "errors": [f"验证过程中发生异常: {e}"],
                        "warnings": []
                    }
                    results[str(yaml_file)] = error_result
                    print(f"⚠️ 跳过文件: {e}")
            
            batch_result = {
                "total_files": len(yaml_files),
                "valid_files": valid_count,
                "invalid_files": invalid_count,
                "skipped_files": skipped_count,
                "results": results,
                "summary": self._generate_batch_summary(valid_count, invalid_count, skipped_count, len(yaml_files))
            }
            
            return batch_result
            
        except Exception as e:
            return {
                "total_files": 0,
                "valid_files": 0,
                "invalid_files": 0,
                "skipped_files": 0,
                "results": {},
                "summary": f"批量验证失败: {e}"
            }

    def _generate_batch_summary(self, valid: int, invalid: int, skipped: int, total: int) -> str:
        """生成批量验证摘要"""
        success_rate = (valid / total * 100) if total > 0 else 0
        
        summary_parts = []
        summary_parts.append(f"总计: {total} 个文件")
        summary_parts.append(f"通过: {valid} 个")
        summary_parts.append(f"失败: {invalid} 个")
        if skipped > 0:
            summary_parts.append(f"跳过: {skipped} 个")
        summary_parts.append(f"成功率: {success_rate:.1f}%")
        
        return " | ".join(summary_parts)

    def validate_config(self, config_file: str) -> Dict[str, Any]:
        """验证配置文件"""
        config_path = Path(config_file)
        if not config_path.exists():
            return {
                "valid": False,
                "errors": [f"配置文件不存在: {config_file}"],
                "warnings": []
            }
        
        self.log(f"🔍 开始验证配置文件: {config_file}")
        
        # 重置验证结果
        self.validation_errors = []
        self.validation_warnings = []
        
        try:
            # 加载配置文件
            with open(config_path, 'r', encoding='utf-8') as f:
                config_content = yaml.safe_load(f)
            
            # 检查基本结构
            if not isinstance(config_content, dict):
                self.validation_errors.append("配置文件根节点必须是对象类型")
                return self._build_result()
            
            # 获取kind字段确定schema类型
            kind = config_content.get('kind')
            if not kind:
                self.validation_errors.append("配置文件缺少必填的'kind'字段")
                return self._build_result()
            
            # 查找对应的schema定义
            schema_def = self._find_schema_by_kind(kind)
            if not schema_def:
                self.validation_errors.append(f"未找到与kind '{kind}' 对应的schema定义")
                return self._build_result()
            
            self.log(f"📋 使用schema: {schema_def['name']}")
            
            # 验证配置结构
            self._validate_config_against_schema(config_content, schema_def)
            
            return self._build_result()
            
        except yaml.YAMLError as e:
            self.validation_errors.append(f"YAML文件格式错误: {e}")
            return self._build_result()
        except Exception as e:
            self.validation_errors.append(f"验证过程中发生错误: {e}")
            return self._build_result()
    
    def _find_schema_by_kind(self, kind: str) -> Optional[Dict[str, Any]]:
        """根据kind字段查找对应的schema定义"""
        # kind可能带有下划线，而schema名称可能是连字符形式
        # 例如：metric_set -> metric-set
        kind_variations = [
            kind,
            kind.replace('_', '-'),
            kind.replace('-', '_')
        ]
        
        for variation in kind_variations:
            if variation in self.schemas:
                return self.schemas[variation]
        
        return None
    
    def _validate_config_against_schema(self, config: Dict[str, Any], schema: Dict[str, Any]):
        """根据schema定义验证配置"""
        if 'versions' not in schema:
            self.validation_errors.append("Schema定义缺少versions字段")
            return
        
        # 使用第一个版本进行验证（通常是最新版本）
        version = schema['versions'][0] if schema['versions'] else None
        if not version or 'spec' not in version:
            self.validation_errors.append("Schema定义缺少有效的spec")
            return
        
        spec = version['spec']
        self.log(f"📝 使用schema版本: {version.get('name', 'unknown')}")
        
        # 保存当前配置以供字段引用使用
        self._current_config = config
        
        # 验证配置文件的每个字段
        if 'properties' in spec:
            self._validate_config_properties(config, spec['properties'], "")
    
    def _validate_config_properties(self, config: Dict[str, Any], spec_properties: Dict[str, Any], path: str):
        """验证配置文件的属性"""
        # 检查配置中的未定义字段
        for key in config.keys():
            if key not in spec_properties and key != 'kind':  # kind字段是特殊的，不在spec中定义
                field_path = f"{path}.{key}" if path else key
                self.validation_warnings.append(f"字段 '{field_path}' 未在schema中定义")
        
        # 验证每个定义的属性
        for prop_name, prop_spec in spec_properties.items():
            prop_path = f"{path}.{prop_name}" if path else prop_name
            
            # 检查是否为必填字段
            is_required = self._is_field_required(prop_spec)
            
            if prop_name in config:
                # 验证字段值
                self._validate_config_field(config[prop_name], prop_spec, prop_path)
            elif is_required:
                self.validation_errors.append(f"缺少必填字段 '{prop_path}'")
    
    def _is_field_required(self, field_spec: Dict[str, Any]) -> bool:
        """判断字段是否必填"""
        if 'constraint' in field_spec and field_spec['constraint'] is not None:
            constraint = field_spec['constraint']
            if 'required' in constraint:
                return constraint['required']
        return False
    
    def _validate_config_field(self, value: Any, spec: Dict[str, Any], path: str):
        """验证配置文件的单个字段"""
        # 创建结果对象用于收集错误和警告
        result = {
            "errors": self.validation_errors,
            "warnings": self.validation_warnings
        }
        
        # 使用父类的验证方法
        self._validate_field_with_spec(value, spec, path, result)
    
    def _validate_field_with_spec(self, value: Any, spec: Dict[str, Any], path: str, result: Dict[str, Any]):
        """使用schema规范验证字段值"""
        # 检查 release_stage
        if 'release_stage' in spec:
            release_stage = spec['release_stage']
            if release_stage == 'experimental':
                result["warnings"].append(f"字段 '{path}' 处于实验阶段 (experimental)，可能会有破坏性变更")
            elif release_stage == 'deprecated':
                result["warnings"].append(f"字段 '{path}' 已被弃用 (deprecated)，建议使用替代方案")
        
        # 获取字段类型
        field_type = self._get_field_type(spec)
        
        if field_type:
            # 验证基本类型
            if not self._check_type(value, field_type):
                result["errors"].append(f"路径 '{path}': 期望类型 {field_type}，实际类型 {type(value).__name__}")
                return
        
        # 验证约束条件
        if 'constraint' in spec and spec['constraint'] is not None:
            self._validate_constraints(value, spec['constraint'], path, result)
        
        # 验证semantic_string类型
        if field_type == 'semantic_string':
            self._validate_semantic_string(value, path, result)
        
        # 递归验证嵌套结构
        if field_type == 'object' and 'properties' in spec and isinstance(value, dict):
            # 使用 _validate_config_properties 来验证嵌套对象，这样可以检查未定义字段
            self._validate_config_properties(value, spec['properties'], path)
        
        # 验证数组元素
        elif field_type == 'array' and isinstance(value, list):
            if 'constraint' in spec and spec['constraint'] is not None:
                constraint = spec['constraint']
                if 'array' in constraint and 'item' in constraint['array']:
                    item_spec = constraint['array']['item']
                    for i, item in enumerate(value):
                        self._validate_field_with_spec(
                            item, 
                            item_spec, 
                            f"{path}[{i}]", 
                            result
                        )
                
                # 验证数组长度
                if 'min_size' in constraint and len(value) < constraint['min_size']:
                    result["errors"].append(f"路径 '{path}': 数组长度 {len(value)} 小于最小值 {constraint['min_size']}")
                if 'max_size' in constraint and len(value) > constraint['max_size']:
                    result["errors"].append(f"路径 '{path}': 数组长度 {len(value)} 超过最大值 {constraint['max_size']}")
    
    def _get_field_type(self, spec: Dict[str, Any]) -> Optional[str]:
        """从spec中获取字段类型"""
        # 优先从spec的type字段获取
        if 'type' in spec:
            return spec['type']
        
        # 其次从constraint中获取
        if 'constraint' in spec and spec['constraint'] is not None:
            if 'type' in spec['constraint']:
                return spec['constraint']['type']
        
        return None
    
    def _build_result(self) -> Dict[str, Any]:
        """构建验证结果"""
        return {
            "valid": len(self.validation_errors) == 0,
            "errors": self.validation_errors,
            "warnings": self.validation_warnings
        }
    
    def generate_report(self, config_file: str, result: Dict[str, Any]) -> str:
        """生成验证报告"""
        report = []
        report.append(f"# 配置文件验证报告")
        report.append(f"**文件**: {config_file}")
        report.append(f"**验证时间**: {self._get_current_time()}")
        report.append("")
        
        if result["valid"]:
            report.append("## ✅ 验证结果")
            report.append("配置文件验证通过！")
        else:
            report.append("## ❌ 验证结果")
            report.append("配置文件验证失败。")
        
        if result["errors"]:
            report.append(f"\n## 🚨 错误 ({len(result['errors'])} 个)")
            for i, error in enumerate(result["errors"], 1):
                report.append(f"{i}. {error}")
        
        if result["warnings"]:
            report.append(f"\n## ⚠️ 警告 ({len(result['warnings'])} 个)")
            for i, warning in enumerate(result["warnings"], 1):
                report.append(f"{i}. {warning}")
        
        if not result["errors"] and not result["warnings"]:
            report.append("\n## 🎉 总结")
            report.append("配置文件完全符合schema规范，没有发现任何问题。")
        
        return "\n".join(report)
    
    def _get_current_time(self) -> str:
        """获取当前时间字符串"""
        from datetime import datetime
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    def generate_batch_report(self, directory: str, batch_result: Dict[str, Any]) -> str:
        """生成批量验证报告"""
        report = []
        report.append(f"# 批量配置文件验证报告")
        report.append(f"**目录**: {directory}")
        report.append(f"**验证时间**: {self._get_current_time()}")
        report.append("")
        
        # 总体统计
        report.append("## 📊 验证统计")
        report.append(f"- 总文件数: {batch_result['total_files']}")
        report.append(f"- 验证通过: {batch_result['valid_files']}")
        report.append(f"- 验证失败: {batch_result['invalid_files']}")
        if batch_result['skipped_files'] > 0:
            report.append(f"- 跳过文件: {batch_result['skipped_files']}")
        
        success_rate = (batch_result['valid_files'] / batch_result['total_files'] * 100) if batch_result['total_files'] > 0 else 0
        report.append(f"- 成功率: {success_rate:.1f}%")
        report.append("")
        
        # 验证结果概览
        if batch_result['total_files'] > 0:
            if batch_result['valid_files'] == batch_result['total_files']:
                report.append("## ✅ 验证结果")
                report.append("所有配置文件验证通过！")
            else:
                report.append("## ❌ 验证结果")
                report.append("部分配置文件验证失败。")
        
        # 详细结果
        if batch_result['results']:
            # 失败的文件
            failed_files = [(file, result) for file, result in batch_result['results'].items() if not result['valid']]
            if failed_files:
                report.append(f"\n## 🚨 验证失败的文件 ({len(failed_files)} 个)")
                for file_path, result in failed_files:
                    report.append(f"\n### {file_path}")
                    if result['errors']:
                        report.append("**错误:**")
                        for error in result['errors']:
                            report.append(f"- {error}")
                    if result['warnings']:
                        report.append("**警告:**")
                        for warning in result['warnings']:
                            report.append(f"- {warning}")
            
            # 成功但有警告的文件
            warning_files = [(file, result) for file, result in batch_result['results'].items() 
                           if result['valid'] and result['warnings']]
            if warning_files:
                report.append(f"\n## ⚠️ 有警告的文件 ({len(warning_files)} 个)")
                for file_path, result in warning_files:
                    report.append(f"\n### {file_path}")
                    report.append("**警告:**")
                    for warning in result['warnings']:
                        report.append(f"- {warning}")
            
            # 完全通过的文件
            perfect_files = [(file, result) for file, result in batch_result['results'].items() 
                           if result['valid'] and not result['warnings']]
            if perfect_files:
                report.append(f"\n## ✅ 完全通过的文件 ({len(perfect_files)} 个)")
                for file_path, result in perfect_files:
                    report.append(f"- {file_path}")
        
        if batch_result['total_files'] == 0:
            report.append("\n## ℹ️ 说明")
            report.append("在指定目录中未找到YAML配置文件。")
        elif batch_result['valid_files'] == batch_result['total_files'] and all(not result['warnings'] for result in batch_result['results'].values()):
            report.append("\n## 🎉 总结")
            report.append("所有配置文件都完全符合schema规范，没有发现任何问题。")
        
        return "\n".join(report)

    def _validate_constraints(self, data: Any, constraints: Dict[str, Any], path: str, result: Dict[str, Any]):
        """验证数据是否满足约束条件"""
        # 调用父类的基础约束验证
        super()._validate_constraints(data, constraints, path, result)
        
        # 验证高级约束
        if 'advanced' in constraints:
            self._validate_advanced_constraint(data, constraints['advanced'], path, result)

    def _validate_advanced_constraint(self, data: Any, advanced_constraint: Dict[str, Any], path: str, result: Dict[str, Any]):
        """验证高级约束条件"""
        try:
            # 处理表达式约束
            if 'expression' in advanced_constraint:
                expression = advanced_constraint['expression']
                # 构建上下文数据，用于字段引用
                context = self._build_context_data(data, path)
                # 求值表达式
                if not self._evaluate_expression(expression, context, path, result):
                    result["errors"].append(f"路径 '{path}': 不满足高级约束条件")
            
                
        except Exception as e:
            result["errors"].append(f"路径 '{path}': 高级约束验证失败 - {e}")

    def _build_context_data(self, current_value: Any, current_path: str) -> Dict[str, Any]:
        """构建上下文数据，用于字段引用"""
        # 这是一个简化版本，实际应该从整个配置对象中构建
        # 目前只处理当前值
        context = {
            "current_value": current_value,
            "current_path": current_path,
            # 这里需要访问整个配置对象，但由于当前架构限制，先简化处理
            "root_config": getattr(self, '_current_config', {}),
        }
        return context

    def _evaluate_expression(self, expression: Dict[str, Any], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> bool:
        """求值表达式"""
        if not isinstance(expression, dict) or 'operator' not in expression:
            result["warnings"].append(f"路径 '{path}': 无效的表达式格式")
            return True
        
        operator = expression['operator']
        
        # 逻辑操作符
        if operator == 'and':
            return self._evaluate_and(expression.get('conditions', []), context, path, result)
        elif operator == 'or':
            return self._evaluate_or(expression.get('conditions', []), context, path, result)
        elif operator == 'not':
            return self._evaluate_not(expression.get('conditions', []), context, path, result)
        
        # 比较操作符
        elif operator in ['eq', 'ne', 'gt', 'ge', 'lt', 'le', 'in', 'nin', 'exists', 'regex']:
            return self._evaluate_comparison(operator, expression, context, path, result)
        
        else:
            result["warnings"].append(f"路径 '{path}': 不支持的操作符 '{operator}'")
            return True

    def _evaluate_and(self, conditions: List[Dict[str, Any]], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> bool:
        """求值AND表达式"""
        if not conditions:
            return True
        
        for i, condition in enumerate(conditions):
            if not self._evaluate_expression(condition, context, f"{path}.and[{i}]", result):
                return False
        return True

    def _evaluate_or(self, conditions: List[Dict[str, Any]], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> bool:
        """求值OR表达式"""
        if not conditions:
            return True
        
        for i, condition in enumerate(conditions):
            if self._evaluate_expression(condition, context, f"{path}.or[{i}]", result):
                return True
        return False

    def _evaluate_not(self, conditions: List[Dict[str, Any]], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> bool:
        """求值NOT表达式"""
        if not conditions:
            return True
        
        # NOT 操作符通常只有一个条件
        if len(conditions) != 1:
            result["warnings"].append(f"路径 '{path}': NOT操作符应该只有一个条件")
            return True
        
        return not self._evaluate_expression(conditions[0], context, f"{path}.not", result)

    def _evaluate_comparison(self, operator: str, expression: Dict[str, Any], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> bool:
        """求值比较表达式"""
        left_operand = expression.get('left')
        right_operand = expression.get('right')
        
        if not left_operand or not right_operand:
            result["warnings"].append(f"路径 '{path}': 比较操作符 '{operator}' 缺少操作数")
            return True
        
        # 获取操作数的值
        left_value = self._get_operand_value(left_operand, context, path, result)
        right_value = self._get_operand_value(right_operand, context, path, result)
        
        # 执行比较操作
        try:
            if operator == 'eq':
                return left_value == right_value
            elif operator == 'ne':
                return left_value != right_value
            elif operator == 'gt':
                return self._compare_values(left_value, right_value, lambda x, y: x > y)
            elif operator == 'ge':
                return self._compare_values(left_value, right_value, lambda x, y: x >= y)
            elif operator == 'lt':
                return self._compare_values(left_value, right_value, lambda x, y: x < y)
            elif operator == 'le':
                return self._compare_values(left_value, right_value, lambda x, y: x <= y)
            elif operator == 'in':
                if isinstance(right_value, (list, tuple)):
                    return left_value in right_value
                elif isinstance(right_value, str):
                    return str(left_value) in right_value
                else:
                    return False
            elif operator == 'nin':
                if isinstance(right_value, (list, tuple)):
                    return left_value not in right_value
                elif isinstance(right_value, str):
                    return str(left_value) not in right_value
                else:
                    return True
            elif operator == 'exists':
                # exists 检查左操作数是否存在（非None且非空字符串）
                return left_value is not None and left_value != ""
            elif operator == 'regex':
                import re
                pattern = str(right_value)
                text = str(left_value) if left_value is not None else ""
                return bool(re.search(pattern, text))
            else:
                result["warnings"].append(f"路径 '{path}': 不支持的比较操作符 '{operator}'")
                return True
                
        except Exception as e:
            result["warnings"].append(f"路径 '{path}': 比较操作失败 - {e}")
            return True

    def _compare_values(self, left: Any, right: Any, comparison_func) -> bool:
        """比较两个值（处理类型转换）"""
        try:
            # 尝试转换为数字进行比较
            if isinstance(left, str) and isinstance(right, str):
                # 如果都是字符串，尝试转换为数字
                try:
                    left_num = float(left)
                    right_num = float(right)
                    return comparison_func(left_num, right_num)
                except ValueError:
                    # 如果不能转换为数字，按字符串比较
                    return comparison_func(left, right)
            elif isinstance(left, (int, float)) and isinstance(right, (int, float)):
                return comparison_func(left, right)
            elif isinstance(left, str) and isinstance(right, (int, float)):
                try:
                    left_num = float(left)
                    return comparison_func(left_num, right)
                except ValueError:
                    return False
            elif isinstance(left, (int, float)) and isinstance(right, str):
                try:
                    right_num = float(right)
                    return comparison_func(left, right_num)
                except ValueError:
                    return False
            else:
                # 其他情况直接比较
                return comparison_func(left, right)
        except:
            return False

    def _get_operand_value(self, operand: Dict[str, Any], context: Dict[str, Any], path: str, result: Dict[str, Any]) -> Any:
        """获取操作数的值"""
        if not isinstance(operand, dict) or 'type' not in operand or 'value' not in operand:
            result["warnings"].append(f"路径 '{path}': 无效的操作数格式")
            return None
        
        operand_type = operand['type']
        operand_value = operand['value']
        
        if operand_type == 'literal':
            # 字面值，直接返回
            return operand_value
        elif operand_type == 'field':
            # 字段引用，需要解析路径
            return self._resolve_field_reference(operand_value, context, path, result)
        else:
            result["warnings"].append(f"路径 '{path}': 不支持的操作数类型 '{operand_type}'")
            return None

    def _resolve_field_reference(self, field_path: str, context: Dict[str, Any], path: str, result: Dict[str, Any]) -> Any:
        """解析字段引用"""
        try:
            if field_path == "$current":
                # 当前字段值
                return context.get("current_value")
            elif field_path == "$current.length":
                # 当前字段长度
                current_value = context.get("current_value")
                if isinstance(current_value, (list, str, dict)):
                    return len(current_value)
                else:
                    return None
            elif field_path.startswith("$parent."):
                # 父级字段引用
                field_name = field_path[8:]  # 去掉 "$parent." 前缀
                result["warnings"].append(f"路径 '{path}': 暂不支持父级字段引用 '{field_path}'，需要完整的上下文实现")
                return None
            elif field_path.startswith("$root."):
                # 根对象字段引用
                field_name = field_path[6:]  # 去掉 "$root." 前缀
                root_config = context.get("root_config", {})
                return self._get_nested_field_value(root_config, field_name)
            elif field_path.startswith("$sibling."):
                # 同级字段引用
                field_name = field_path[9:]  # 去掉 "$sibling." 前缀
                result["warnings"].append(f"路径 '{path}': 暂不支持同级字段引用 '{field_path}'，需要完整的上下文实现")
                return None
            else:
                # 向后兼容旧语法
                if field_path == ".":
                    return context.get("current_value")
                elif field_path.startswith("../"):
                    result["warnings"].append(f"路径 '{path}': 建议使用新语法 '$parent.字段名' 替代 '{field_path}'")
                    return None
                elif field_path.startswith("$."):
                    if field_path == "$.length":
                        current_value = context.get("current_value")
                        if isinstance(current_value, (list, str, dict)):
                            return len(current_value)
                        else:
                            return None
                    else:
                        result["warnings"].append(f"路径 '{path}': 建议使用新语法 '$root.字段名' 替代 '{field_path}'")
                        return None
                else:
                    result["warnings"].append(f"路径 '{path}': 不支持的字段引用语法 '{field_path}'")
                    return None
        except Exception as e:
            result["warnings"].append(f"路径 '{path}': 字段引用解析失败 '{field_path}' - {e}")
            return None

    def _get_nested_field_value(self, obj: Dict[str, Any], field_path: str) -> Any:
        """获取嵌套字段的值"""
        try:
            if not isinstance(obj, dict):
                return None
            
            # 支持点号分隔的嵌套路径，如 "spec.user_type"
            parts = field_path.split('.')
            current = obj
            
            for part in parts:
                if isinstance(current, dict) and part in current:
                    current = current[part]
                else:
                    return None
            
            return current
        except:
            return None


def main():
    """主函数"""
    parser = argparse.ArgumentParser(
        description="UModel 配置文件验证器",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
使用示例:
  # 验证单个文件
  python umodel_validator.py config.yaml
  
  # 批量验证目录（递归）
  python umodel_validator.py --batch examples/
  
  # 批量验证目录（非递归）
  python umodel_validator.py --batch examples/ --no-recursive
  
  # 启用控制台日志输出
  python umodel_validator.py --console-log config.yaml
        """
    )
    
    parser.add_argument('target', nargs='?', help='要验证的配置文件路径')
    parser.add_argument('--batch', '-b', metavar='DIR', help='批量验证指定目录下的所有YAML文件')
    parser.add_argument('--no-recursive', action='store_true', help='批量验证时不递归搜索子目录')
    parser.add_argument('--console-log', action='store_true', help='启用控制台日志输出')
    parser.add_argument('--log-level', choices=['DEBUG', 'INFO', 'WARNING', 'ERROR'], 
                       default='INFO', help='设置日志级别 (默认: INFO)')
    parser.add_argument('--schemas-dir', '-s', default='expanded_schemas', 
                       help='展开后的schema定义目录路径 (默认: expanded_schemas)')
    parser.add_argument('--base-schema', default='schemas/base.yaml',
                       help='base.yaml文件路径 (默认: schemas/base.yaml)')
    
    args = parser.parse_args()
    
    # 配置日志
    if args.console_log:
        log_level = getattr(logging, args.log_level)
        logging.basicConfig(
            level=log_level,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
            handlers=[
                logging.StreamHandler(sys.stdout)
            ]
        )
        print(f"✅ 控制台日志已启用，级别: {args.log_level}")
    
    # 参数验证
    if not args.target and not args.batch:
        parser.print_help()
        print("\n❌ 错误: 必须指定要验证的文件或使用 --batch 选项")
        sys.exit(1)
    
    if args.target and args.batch:
        print("❌ 错误: 不能同时指定文件和 --batch 选项")
        sys.exit(1)
    
    print("🚀 UModel 配置文件验证器启动")
    print("=" * 50)
    
    try:
        # 创建验证器
        validator = ConfigValidator(args.schemas_dir, args.base_schema, console_log=args.console_log)
        
        if args.batch:
            # 批量验证模式
            recursive = not args.no_recursive
            batch_result = validator.validate_batch(args.batch, recursive)
            
            # 生成批量报告
            report = validator.generate_batch_report(args.batch, batch_result)
            
            # 输出结果
            print("\n" + "=" * 60)
            print(report)
            
            # 保存报告到文件
            report_file = f"batch_validation_report_{Path(args.batch).name}.md"
            with open(report_file, 'w', encoding='utf-8') as f:
                f.write(report)
            
            print(f"\n📄 批量验证报告已保存到: {report_file}")
            
            # 返回适当的退出码
            sys.exit(0 if batch_result['invalid_files'] == 0 else 1)
            
        else:
            # 单文件验证模式
            config_file = args.target
            result = validator.validate_config(config_file)
            
            # 生成报告
            report = validator.generate_report(config_file, result)
            
            # 输出结果
            print("\n" + "=" * 50)
            print(report)
            
            # 保存报告到文件
            report_file = f"{Path(config_file).stem}_validation_report.md"
            with open(report_file, 'w', encoding='utf-8') as f:
                f.write(report)
            
            print(f"\n📄 验证报告已保存到: {report_file}")
            
            # 返回适当的退出码
            sys.exit(0 if result["valid"] else 1)
        
    except Exception as e:
        print(f"\n❌ 验证过程中发生错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main() 