#!/usr/bin/env python3
"""
UModel Schema Go SDK Generator V2

这个脚本基于原始的schemas目录，生成高复用的Go SDK代码。
利用Go的嵌入（embedding）特性来实现继承关系，生成更优雅的代码结构。

主要改进：
1. 直接从schemas目录读取，保留原始的继承关系
2. 为共享类型生成独立的struct定义
3. 使用Go的嵌入特性实现extends
4. 生成更简洁、高复用的代码
"""

import os
import yaml
import json
from pathlib import Path
from typing import Dict, Any, List, Optional, Set, Tuple
from collections import defaultdict, OrderedDict
import re


class GoCodeGeneratorV2:
    def __init__(self, schemas_dir: str, output_dir: str):
        self.schemas_dir = Path(schemas_dir)
        self.output_dir = Path(output_dir)
        
        # 目录结构
        self.base_schema_path = self.schemas_dir / "base.yaml"
        self.core_dir = self.schemas_dir / "core"
        self.includes_dir = self.schemas_dir / "includes"
        
        # 存储所有的类型定义
        self.type_registry: Dict[str, Dict[str, Any]] = {}  # type_name:version -> definition
        self.schema_registry: Dict[str, Dict[str, Any]] = {}  # schema_name -> schema_content
        
        # 存储生成的Go类型名称映射
        self.go_type_mapping: Dict[str, str] = {}  # type_ref -> Go类型名
        
        # 记录依赖关系
        self.type_dependencies: Dict[str, Set[str]] = defaultdict(set)  # type -> dependencies
        
        # 存储内联结构体定义
        self.inline_structs: List[Tuple[str, List[str]]] = []
        
        # 基础类型映射
        self.primitive_types = {
            "string": "string",
            "number": "float64",
            "integer": "int64",
            "float": "float64",
            "boolean": "bool",
            "bool": "bool",
            "object": "map[string]interface{}",
            "array": "[]interface{}",
            "map": "map[string]interface{}",
            "any": "interface{}",
            "json": "json.RawMessage",
            "json_object": "map[string]interface{}",
            "json_array": "[]interface{}",
            "time": "time.Time",
            "enum": "string",
        }
        
        # 加载base.yaml
        self.base_schema = self._load_base_schema()
        self.metadata_properties = self.base_schema.get("metadata_properties", {})
        self.additional_types = self.base_schema.get("additional_types", {})
        
    def _load_base_schema(self) -> Dict[str, Any]:
        """加载base.yaml文件"""
        try:
            with open(self.base_schema_path, 'r', encoding='utf-8') as f:
                return yaml.safe_load(f)
        except Exception as e:
            print(f"❌ 无法加载base.yaml: {e}")
            return {}
    
    def load_all_definitions(self):
        """加载所有的schema和类型定义"""
        print("🔍 正在加载schema定义...")
        
        # 1. 从base.yaml加载additional_types
        self._load_additional_types()
        
        # 2. 加载includes目录的共享类型
        self._load_include_types()
        
        # 3. 加载core目录的schema定义
        self._load_core_schemas()
        
        print(f"✅ 已加载 {len(self.type_registry)} 个类型定义")
        print(f"✅ 已加载 {len(self.schema_registry)} 个schema文件")
    
    def _load_additional_types(self):
        """从base.yaml加载additional_types"""
        if self.additional_types and 'types' in self.additional_types:
            types = self.additional_types.get('types', [])
            for type_def in types:
                if 'name' in type_def:
                    type_name = type_def['name']
                    # 对于base.yaml中的类型，默认版本为v1
                    self.type_registry[f"{type_name}:v1"] = {
                        'name': type_name,
                        'version': 'v1',
                        'spec': type_def,
                        'file': 'base'
                    }
                    print(f"📌 已加载base类型: {type_name}")
    
    def _load_include_types(self):
        """加载includes目录的共享类型"""
        if not self.includes_dir.exists():
            return
            
        for yaml_file in self.includes_dir.glob("*.schema.yaml"):
            try:
                with open(yaml_file, 'r', encoding='utf-8') as f:
                    content = yaml.safe_load(f)
                
                if content and 'name' in content:
                    type_name = content['name']
                    if 'versions' in content:
                        for version in content['versions']:
                            version_name = version.get('name', 'v1')
                            type_key = f"{type_name}:{version_name}"
                            self.type_registry[type_key] = {
                                'name': type_name,
                                'version': version_name,
                                'spec': version.get('spec', {}),
                                'file': 'includes'
                            }
                    print(f"📁 已加载include类型: {type_name}")
            except Exception as e:
                print(f"❌ 加载includes文件失败 {yaml_file}: {e}")
    
    def _load_core_schemas(self):
        """加载core目录的schema定义"""
        if not self.core_dir.exists():
            return
            
        for yaml_file in self.core_dir.rglob("*.schema.yaml"):
            try:
                with open(yaml_file, 'r', encoding='utf-8') as f:
                    content = yaml.safe_load(f)
                
                if content and 'name' in content:
                    schema_name = content['name']
                    self.schema_registry[schema_name] = content
                    
                    # 同时注册为类型
                    if 'versions' in content:
                        for version in content['versions']:
                            version_name = version.get('name', 'v1')
                            type_key = f"{schema_name}:{version_name}"
                            self.type_registry[type_key] = {
                                'name': schema_name,
                                'version': version_name,
                                'spec': version.get('spec', {}),
                                'file': 'core'
                            }
                    
                    print(f"📄 已加载schema: {schema_name}")
            except Exception as e:
                print(f"❌ 加载schema文件失败 {yaml_file}: {e}")
    
    def analyze_dependencies(self):
        """分析类型之间的依赖关系"""
        print("\n🔍 分析类型依赖关系...")
        
        for type_key, type_def in self.type_registry.items():
            if isinstance(type_def, dict) and 'spec' in type_def:
                self._analyze_spec_dependencies(type_key, type_def['spec'])
        
        # 打印依赖关系
        for type_key, deps in self.type_dependencies.items():
            if deps:
                print(f"📊 {type_key} 依赖: {', '.join(deps)}")
    
    def _analyze_spec_dependencies(self, type_key: str, spec: Dict[str, Any]):
        """分析spec中的依赖关系"""
        if not isinstance(spec, dict):
            return
            
        # 分析extends依赖
        if 'extends' in spec:
            extends_list = spec['extends']
            if isinstance(extends_list, list):
                for extend_ref in extends_list:
                    self.type_dependencies[type_key].add(extend_ref)
        
        # 分析type_ref依赖
        if 'type_ref' in spec:
            self.type_dependencies[type_key].add(spec['type_ref'])
        
        # 分析properties中的依赖
        if 'properties' in spec:
            for prop_name, prop_spec in spec['properties'].items():
                if isinstance(prop_spec, dict):
                    self._analyze_spec_dependencies(type_key, prop_spec)
    
    def generate_all(self):
        """生成所有的Go代码"""
        print("\n🚀 开始生成Go SDK代码 V2...")
        
        # 创建输出目录
        self.output_dir.mkdir(parents=True, exist_ok=True)
        
        # 1. 加载所有定义
        self.load_all_definitions()
        
        # 2. 分析依赖关系
        self.analyze_dependencies()
        
        # 3. 生成基础类型文件
        self._generate_base_types()
        
        # 4. 生成共享类型文件
        self._generate_shared_types()
        
        # 5. 生成schema文件
        self._generate_schema_files()
        
        # 6. 生成主包文件
        self._generate_main_package()
        
        print("\n✅ Go SDK V2代码生成完成！")
    
    def _generate_base_types(self):
        """生成基础类型定义"""
        content = []
        content.append("// Code generated by schema_go_generator_v2.py. DO NOT EDIT.")
        content.append("")
        content.append("package umodel")
        content.append("")
        content.append("import (")
        content.append('\t"encoding/json"')
        content.append('\t"time"')
        content.append('\t"gopkg.in/yaml.v3"')
        content.append(")")
        content.append("")
        
        # 生成SemanticString类型
        content.append("// SemanticString 支持多语言的语义字符串")
        content.append("type SemanticString struct {")
        content.append("\tZhCN string `json:\"zh_cn,omitempty\" yaml:\"zh_cn,omitempty\"`")
        content.append("\tEnUS string `json:\"en_us,omitempty\" yaml:\"en_us,omitempty\"`")
        content.append("}")
        content.append("")
        
        # 生成SemanticString的String方法
        content.append("// String 返回中文描述，如果没有则返回英文")
        content.append("func (s *SemanticString) String() string {")
        content.append("\tif s.ZhCN != \"\" {")
        content.append("\t\treturn s.ZhCN")
        content.append("\t}")
        content.append("\treturn s.EnUS")
        content.append("}")
        content.append("")
        
        # 生成UnmarshalJSON方法
        content.append("// UnmarshalJSON implements json.Unmarshaler")
        content.append("func (s *SemanticString) UnmarshalJSON(data []byte) error {")
        content.append("\t// 尝试解析为字符串")
        content.append("\tvar str string")
        content.append("\tif err := json.Unmarshal(data, &str); err == nil {")
        content.append("\t\ts.ZhCN = str")
        content.append("\t\ts.EnUS = str")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("")
        content.append("\t// 尝试解析为对象")
        content.append("\ttype Alias SemanticString")
        content.append("\tvar alias Alias")
        content.append("\tif err := json.Unmarshal(data, &alias); err != nil {")
        content.append("\t\treturn err")
        content.append("\t}")
        content.append("\t*s = SemanticString(alias)")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        # 生成UnmarshalYAML方法
        content.append("// UnmarshalYAML implements yaml.Unmarshaler")
        content.append("func (s *SemanticString) UnmarshalYAML(value *yaml.Node) error {")
        content.append("\tif value.Kind == yaml.ScalarNode {")
        content.append("\t\ts.ZhCN = value.Value")
        content.append("\t\ts.EnUS = value.Value")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("")
        content.append("\ttype Alias SemanticString")
        content.append("\tvar alias Alias")
        content.append("\tif err := value.Decode(&alias); err != nil {")
        content.append("\t\treturn err")
        content.append("\t}")
        content.append("\t*s = SemanticString(alias)")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        # 生成UModelObject接口
        content.append("// UModelObject 所有UModel对象的基础接口")
        content.append("type UModelObject interface {")
        content.append("\t// GetKind 获取对象的类型标识")
        content.append("\tGetKind() string")
        content.append("\t// Validate 验证对象是否符合schema定义")
        content.append("\tValidate() error")
        content.append("}")
        content.append("")
        
        # 生成UModelCoreObject接口
        content.append("// UModelCoreObject 所有core目录对象的接口")
        content.append("type UModelCoreObject interface {")
        content.append("\tUModelObject")
        content.append("\t// GetSchema 获取对象的Schema信息")
        content.append("\tGetSchema() *SchemaV1")
        content.append("\t// GetMetadata 获取对象的Metadata信息")
        content.append("\tGetMetadata() *MetadataV1")
        content.append("}")
        content.append("")
        
        # 生成UModelLinkObject接口
        content.append("// UModelLinkObject 所有link类型对象的接口")
        content.append("type UModelLinkObject interface {")
        content.append("\tUModelCoreObject")
        content.append("\t// GetSrc 获取源对象信息")
        content.append("\tGetSrc() *LinkEndpoint")
        content.append("\t// GetDest 获取目标对象信息")
        content.append("\tGetDest() *LinkEndpoint")
        content.append("}")
        content.append("")
        
        # 生成LinkEndpoint结构体
        content.append("// LinkEndpoint 表示Link的端点信息")
        content.append("type LinkEndpoint struct {")
        content.append('\tDomain string `json:"domain" yaml:"domain"`')
        content.append('\tKind   string `json:"kind" yaml:"kind"`')
        content.append('\tName   string `json:"name" yaml:"name"`')
        content.append('\tFilter string `json:"filter,omitempty" yaml:"filter,omitempty"`')
        content.append("}")
        content.append("")
        
        # 添加init函数以避免unused import错误
        content.append("// init 确保所有导入的包都被使用")
        content.append("func init() {")
        content.append("\t_ = time.Now")
        content.append("}")
        content.append("")
        
        # 写入文件
        output_file = self.output_dir / "base_types.go"
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(content))
        
        print(f"✅ 生成基础类型文件: {output_file.name}")
    
    def _generate_shared_types(self):
        """生成共享类型定义"""
        content = []
        content.append("// Code generated by schema_go_generator_v2.py. DO NOT EDIT.")
        content.append("")
        content.append("package umodel")
        content.append("")
        content.append("import (")
        content.append('\t"encoding/json"')
        content.append('\t"time"')
        content.append(")")
        content.append("")
        
        # 初始化内联结构体列表
        self.inline_structs = []
        
        # 按照依赖顺序生成类型
        generated = set()
        
        # 先生成没有依赖的类型
        for type_key in self.type_registry:
            if type_key not in self.type_dependencies or not self.type_dependencies[type_key]:
                if type_key not in generated:
                    self._generate_type_struct(content, type_key, generated)
        
        # 再生成有依赖的类型
        max_iterations = len(self.type_registry) * 2
        iteration = 0
        while len(generated) < len(self.type_registry) and iteration < max_iterations:
            iteration += 1
            for type_key in self.type_registry:
                if type_key not in generated:
                    # 检查所有依赖是否已生成
                    deps = self.type_dependencies.get(type_key, set())
                    if all(dep in generated or self._resolve_type_ref(dep) is None for dep in deps):
                        self._generate_type_struct(content, type_key, generated)
        
        # 生成所有内联结构体
        for inline_struct_name, inline_struct_content in self.inline_structs:
            content.extend(inline_struct_content)
        
        # 添加init函数以避免unused import错误
        content.append("// init 确保所有导入的包都被使用")
        content.append("func init() {")
        content.append("\t_ = json.Marshal")
        content.append("\t_ = time.Now")
        content.append("}")
        content.append("")
        
        # 写入文件
        output_file = self.output_dir / "shared_types.go"
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(content))
        
        print(f"✅ 生成共享类型文件: {output_file.name}")
    
    def _generate_type_struct(self, content: List[str], type_key: str, generated: Set[str]):
        """生成单个类型的struct定义"""
        type_def = self.type_registry.get(type_key)
        if not type_def or not isinstance(type_def, dict):
            return
            
        # 跳过schema类型，它们会在单独的文件中生成
        if type_def.get('file') == 'core':
            return
            
        # 生成Go类型名
        go_type_name = self._get_go_type_name(type_key)
        self.go_type_mapping[type_key] = go_type_name
        
        # 添加到已生成集合
        generated.add(type_key)
        
        # 获取spec
        spec = type_def.get('spec', type_def)
        
        # 生成struct注释
        desc = self._get_description(spec.get('description', ''))
        if desc:
            content.append(f"// {go_type_name} {desc}")
        else:
            content.append(f"// {go_type_name} represents {type_def.get('name', type_key)}")
        
        content.append(f"type {go_type_name} struct {{")
        
        # 处理extends（使用Go的嵌入）
        if 'extends' in spec and isinstance(spec['extends'], list):
            for extend_ref in spec['extends']:
                parent_type = self._get_go_type_for_ref(extend_ref, is_embed=True)
                if parent_type and parent_type != "interface{}":
                    # 使用嵌入，添加inline标签以支持YAML解析
                    content.append(f'\t{parent_type} `yaml:",inline"` // 嵌入父类型')
        
        # 处理properties
        if 'properties' in spec and isinstance(spec['properties'], dict):
            # 收集已经从父类继承的属性
            inherited_props = self._get_inherited_properties(spec)
            
            for prop_name, prop_spec in spec['properties'].items():
                # 跳过已经从父类继承的属性
                if prop_name not in inherited_props:
                    self._generate_struct_field(content, prop_name, prop_spec, go_type_name)
        
        content.append("}")
        content.append("")
        
        # 生成Validate方法
        self._generate_validate_method(content, go_type_name, spec)
        
        # 生成GetKind方法
        self._generate_get_kind_method(content, go_type_name, type_key)
    
    def _get_inherited_properties(self, spec: Dict[str, Any]) -> Set[str]:
        """获取从父类继承的属性名集合"""
        inherited = set()
        
        if 'extends' in spec and isinstance(spec['extends'], list):
            for extend_ref in spec['extends']:
                parent_def = self._resolve_type_ref(extend_ref)
                if parent_def and 'spec' in parent_def:
                    parent_spec = parent_def['spec']
                    if 'properties' in parent_spec:
                        inherited.update(parent_spec['properties'].keys())
                    # 递归获取父类的父类属性
                    inherited.update(self._get_inherited_properties(parent_spec))
        
        return inherited
    
    def _generate_schema_files(self):
        """为每个schema生成独立的文件"""
        for schema_name, schema_content in self.schema_registry.items():
            self._generate_single_schema_file(schema_name, schema_content)
    
    def _generate_single_schema_file(self, schema_name: str, schema_content: Dict[str, Any]):
        """生成单个schema文件"""
        content = []
        content.append("// Code generated by schema_go_generator_v2.py. DO NOT EDIT.")
        content.append("")
        content.append("package umodel")
        content.append("")
        content.append("import (")
        content.append('\t"encoding/json"')
        content.append('\t"time"')
        content.append(")")
        content.append("")
        
        # 初始化内联结构体列表
        self.inline_structs = []
        
        # 生成每个版本的struct
        if 'versions' in schema_content:
            for version in schema_content['versions']:
                version_name = version.get('name', 'v1')
                spec = version.get('spec', {})
                
                # 生成struct名称
                type_key = f"{schema_name}:{version_name}"
                go_type_name = self._get_go_type_name(type_key)
                self.go_type_mapping[type_key] = go_type_name
                
                # 生成struct定义
                self._generate_schema_struct(content, go_type_name, spec, schema_name)
                
                # 生成UModelCoreObject接口方法
                self._generate_core_object_methods(content, go_type_name, schema_name)
                
                # 如果是link类型，生成UModelLinkObject接口方法
                if schema_name.endswith('_link'):
                    self._generate_link_object_methods(content, go_type_name, spec)
        
        # 生成所有内联结构体
        for inline_struct_name, inline_struct_content in self.inline_structs:
            content.extend(inline_struct_content)
        
        # 添加init函数以避免unused import错误
        content.append("// init 确保所有导入的包都被使用")
        content.append("func init() {")
        content.append("\t_ = json.Marshal")
        content.append("\t_ = time.Now")
        content.append("}")
        content.append("")
        
        # 写入文件
        output_file = self.output_dir / f"{schema_name}.go"
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(content))
        
        print(f"✅ 生成schema文件: {output_file.name}")
    
    def _generate_schema_struct(self, content: List[str], struct_name: str, spec: Dict[str, Any], schema_name: str):
        """生成schema的struct定义"""
        # 生成struct注释
        desc = self._get_description(spec.get('description', ''))
        if desc:
            content.append(f"// {struct_name} {desc}")
        else:
            content.append(f"// {struct_name} represents {schema_name}")
        
        content.append(f"type {struct_name} struct {{")
        
        # 检查顶层的 extends（使用Go的嵌入）
        if 'extends' in spec and isinstance(spec['extends'], list):
            for extend_ref in spec['extends']:
                parent_type = self._get_go_type_for_ref(extend_ref, is_embed=True)
                if parent_type and parent_type != "interface{}":
                    content.append(f'\t{parent_type} `yaml:",inline"` // 嵌入 {extend_ref}')
        
        # 处理type_ref（使用Go的嵌入）
        if 'type_ref' in spec:
            ref_type = self._get_go_type_for_ref(spec['type_ref'], is_embed=True)
            if ref_type and ref_type != "interface{}":
                content.append(f'\t{ref_type} `yaml:",inline"` // 引用 {spec["type_ref"]}')
        
        # 处理properties
        if 'properties' in spec and isinstance(spec['properties'], dict):
            # 收集已经从父类继承的属性
            inherited_props = self._get_inherited_properties(spec)
            
            for prop_name, prop_spec in spec['properties'].items():
                # 检查属性是否有 extends
                if isinstance(prop_spec, dict) and 'extends' in prop_spec:
                    # 检查是否同时有自己的properties（需要生成内联结构体）
                    if 'properties' in prop_spec and prop_spec['properties']:
                        # 生成内联结构体，嵌入父类型并包含自己的属性
                        inline_struct_name = f"{struct_name}{self._to_go_field_name(prop_name)}"
                        self._generate_inline_struct_with_extends(inline_struct_name, prop_spec)
                        
                        go_field_name = self._to_go_field_name(prop_name)
                        field_desc = self._get_description(prop_spec.get('description', ''))
                        if field_desc:
                            content.append(f"\t// {go_field_name} {field_desc}")
                        content.append(f'\t{go_field_name} *{inline_struct_name} `json:"{prop_name}" yaml:"{prop_name}"`')
                    else:
                        # 只有继承，没有自己的属性，直接使用父类型
                        for extend_ref in prop_spec['extends']:
                            parent_type = self._get_go_type_for_ref(extend_ref, is_embed=False)
                            if parent_type and parent_type != "interface{}":
                                go_field_name = self._to_go_field_name(prop_name)
                                field_desc = self._get_description(prop_spec.get('description', ''))
                                if field_desc:
                                    content.append(f"\t// {go_field_name} {field_desc}")
                                content.append(f'\t{go_field_name} {parent_type} `json:"{prop_name}" yaml:"{prop_name}"`')
                elif prop_name not in inherited_props:
                    # 正常生成字段
                    self._generate_struct_field(content, prop_name, prop_spec, struct_name)
        
        content.append("}")
        content.append("")
        
        # 生成Validate方法
        self._generate_validate_method(content, struct_name, spec)
        
        # 生成GetKind方法
        self._generate_get_kind_method(content, struct_name, schema_name)
    
    def _generate_inline_struct_with_extends(self, struct_name: str, spec: Dict[str, Any]):
        """生成带有继承的内联结构体"""
        content = []
        
        # 生成struct注释
        desc = self._get_description(spec.get('description', ''))
        if desc:
            content.append(f"// {struct_name} {desc}")
        else:
            content.append(f"// {struct_name} inline struct with extends")
        
        content.append(f"type {struct_name} struct {{")
        
        # 处理extends（使用Go的嵌入）
        if 'extends' in spec and isinstance(spec['extends'], list):
            for extend_ref in spec['extends']:
                parent_type = self._get_go_type_for_ref(extend_ref, is_embed=True)
                if parent_type and parent_type != "interface{}":
                    content.append(f'\t{parent_type} `yaml:",inline"` // 嵌入 {extend_ref}')
        
        # 处理自己的properties
        if 'properties' in spec and isinstance(spec['properties'], dict):
            for prop_name, prop_spec in spec['properties'].items():
                self._generate_struct_field(content, prop_name, prop_spec, struct_name)
        
        content.append("}")
        content.append("")
        
        # 将内联结构体添加到待生成列表
        self.inline_structs.append((struct_name, content))
    
    def _generate_inline_struct_with_properties(self, struct_name: str, spec: Dict[str, Any]):
        """生成带有properties的内联结构体（用于object类型）"""
        content = []
        
        # 生成struct注释
        desc = self._get_description(spec.get('description', ''))
        if desc:
            content.append(f"// {struct_name} {desc}")
        else:
            content.append(f"// {struct_name} inline struct")
        
        content.append(f"type {struct_name} struct {{")
        
        # 处理properties
        if 'properties' in spec and isinstance(spec['properties'], dict):
            for prop_name, prop_spec in spec['properties'].items():
                self._generate_struct_field(content, prop_name, prop_spec, struct_name)
        
        content.append("}")
        content.append("")
        
        # 生成Validate方法
        self._generate_validate_method(content, struct_name, spec)
        
        # 将内联结构体添加到待生成列表
        self.inline_structs.append((struct_name, content))
    
    def _generate_struct_field(self, content: List[str], field_name: str, field_spec: Dict[str, Any], parent_struct_name: str):
        """生成struct字段"""
        if field_spec is None:
            return
            
        # 获取字段描述
        desc = ""
        if isinstance(field_spec, dict) and 'description' in field_spec:
            desc = self._get_description(field_spec['description'])
        
        # 获取Go类型
        go_type = self._get_go_type(field_spec, field_name, parent_struct_name)
        if parent_struct_name == "LokiV100Spec" and field_name == "tls_verify":
            go_type = "*bool"
        
        # 获取字段标签
        json_tag = field_name
        yaml_tag = field_name
        
        # 检查是否必填
        is_required = self._is_required_field(field_spec)
        
        if not is_required:
            json_tag += ",omitempty"
            yaml_tag += ",omitempty"
        
        # 生成字段
        go_field_name = self._to_go_field_name(field_name)
        
        if desc:
            content.append(f"\t// {go_field_name} {desc}")
        content.append(f'\t{go_field_name} {go_type} `json:"{json_tag}" yaml:"{yaml_tag}"`')
    
    def _get_go_type(self, spec: Dict[str, Any], field_name: str = "", parent_struct_name: str = "") -> str:
        """获取字段的Go类型"""
        if not isinstance(spec, dict):
            return "interface{}"
        
        # 处理type_ref
        if 'type_ref' in spec:
            return self._get_go_type_for_ref(spec['type_ref'])
        
        # 处理constraint中的类型
        if 'constraint' in spec and isinstance(spec['constraint'], dict):
            constraint = spec['constraint']
            
            # 处理array约束
            if 'array' in constraint and 'item' in constraint['array']:
                item_spec = constraint['array']['item']
                item_type = self._get_go_type(item_spec, f"{field_name}_item", parent_struct_name)
                return f"[]{item_type}"
            
            # 处理map约束
            if 'map' in constraint:
                value_spec = constraint['map'].get('value', {'type': 'string'})
                value_type = self._get_go_type(value_spec, f"{field_name}_value", parent_struct_name)
                return f"map[string]{value_type}"
        
        # 处理基本类型
        if 'type' in spec:
            base_type = spec['type']
            
            # 检查是否是自定义类型
            if base_type not in self.primitive_types:
                # 尝试查找类型定义
                type_ref = f"{base_type}:v1"
                if type_ref in self.type_registry:
                    return f"*{self._get_go_type_name(type_ref)}"
                
                # 特殊处理semantic_string
                if base_type == 'semantic_string':
                    return "*SemanticString"
            
            # 处理object类型
            if base_type == 'object' and 'properties' in spec:
                # 检查是否是semantic_string的展开形式
                props = spec['properties']
                if isinstance(props, dict) and set(props.keys()) <= {"zh_cn", "en_us"}:
                    return "*SemanticString"
                
                # 生成内联struct
                if field_name and parent_struct_name:
                    inline_struct_name = f"{parent_struct_name}{self._to_go_field_name(field_name)}"
                    self._generate_inline_struct_with_properties(inline_struct_name, spec)
                    return f"*{inline_struct_name}"
                else:
                    return "map[string]interface{}"
            
            return self.primitive_types.get(base_type, "interface{}")
        
        # 如果没有type字段，但有properties，也处理为object
        if 'properties' in spec and isinstance(spec['properties'], dict):
            # 生成内联struct
            if field_name and parent_struct_name:
                inline_struct_name = f"{parent_struct_name}{self._to_go_field_name(field_name)}"
                self._generate_inline_struct_with_properties(inline_struct_name, spec)
                return f"*{inline_struct_name}"
            else:
                return "map[string]interface{}"
        
        return "interface{}"
    
    def _get_go_type_for_ref(self, ref: str, is_embed: bool = False) -> str:
        """获取引用类型对应的Go类型
        
        Args:
            ref: 类型引用
            is_embed: 是否用于嵌入，如果是嵌入则不需要指针
        """
        # 如果没有版本号，添加默认版本
        if ':' not in ref:
            ref = f"{ref}:v1"
        
        # 如果已经在映射中，使用映射的名称
        if ref in self.go_type_mapping:
            go_type_name = self.go_type_mapping[ref]
        elif ref in self.type_registry:
            go_type_name = self._get_go_type_name(ref)
            self.go_type_mapping[ref] = go_type_name
        else:
            return "interface{}"
        
        # 如果是嵌入，直接返回类型名；否则返回指针类型
        return go_type_name if is_embed else f"*{go_type_name}"
    
    def _resolve_type_ref(self, ref: str) -> Optional[Dict[str, Any]]:
        """解析类型引用"""
        # 如果没有版本号，添加默认版本
        if ':' not in ref:
            ref = f"{ref}:v1"
        
        return self.type_registry.get(ref)
    
    def _get_go_type_name(self, type_key: str) -> str:
        """将类型键转换为Go类型名"""
        # 分解type_key
        parts = type_key.split(':')
        if len(parts) == 2:
            name, version = parts
        else:
            name = parts[0]
            version = 'v1'
        
        # 转换为驼峰命名
        name_parts = name.split('_')
        go_name = ''.join(word.capitalize() for word in name_parts)
        
        # 添加版本后缀
        version_suffix = version.replace('.', '').upper()
        return f"{go_name}{version_suffix}"
    
    def _to_go_field_name(self, field_name: str) -> str:
        """将字段名转换为Go字段名"""
        parts = field_name.split('_')
        return ''.join(word.capitalize() for word in parts)
    
    def _is_required_field(self, field_spec: Dict[str, Any]) -> bool:
        """判断字段是否必填"""
        if not isinstance(field_spec, dict):
            return False
            
        if 'constraint' in field_spec:
            constraint = field_spec['constraint']
            if isinstance(constraint, dict):
                return constraint.get('required', False)
        
        return False
    
    def _get_description(self, desc: Any) -> str:
        """获取描述文本，处理换行和限制长度"""
        text = ""
        if isinstance(desc, str):
            text = desc
        elif isinstance(desc, dict):
            # 优先返回中文描述
            if 'zh_cn' in desc:
                text = desc['zh_cn']
            elif 'en_us' in desc:
                text = desc['en_us']
        
        if not text:
            return ""
        
        # 处理换行符，将多行文本合并为单行
        text = text.replace('\n', ' ').replace('\r', ' ')
        
        # 替换连续的空格为单个空格
        text = ' '.join(text.split())
        
        # 限制长度为128字符
        if len(text) > 128:
            # 截断到125个字符，为 "..." 留出空间
            text = text[:125] + "..."
        
        return text
    
    def _generate_validate_method(self, content: List[str], struct_name: str, spec: Dict[str, Any]):
        """生成Validate方法"""
        content.append(f"// Validate 验证{struct_name}是否符合schema定义")
        content.append(f"func (s *{struct_name}) Validate() error {{")
        content.append("\t// TODO: 实现验证逻辑")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
    
    def _generate_get_kind_method(self, content: List[str], struct_name: str, schema_name: str):
        """生成GetKind方法"""
        # 从type_name中提取kind（去掉版本号）
        kind = schema_name.split(':')[0] if ':' in schema_name else schema_name
        
        content.append(f"// GetKind 获取{struct_name}的类型标识")
        content.append(f"func (s *{struct_name}) GetKind() string {{")
        content.append(f'\treturn "{kind}"')
        content.append("}")
        content.append("")
    
    def _generate_core_object_methods(self, content: List[str], struct_name: str, schema_name: str):
        """生成UModelCoreObject接口方法"""
        # 生成GetSchema方法
        content.append(f"// GetSchema 获取{struct_name}的Schema信息")
        content.append(f"func (s *{struct_name}) GetSchema() *SchemaV1 {{")
        content.append("\tif s == nil {")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("\treturn s.Schema")
        content.append("}")
        content.append("")
        
        # 生成GetMetadata方法
        content.append(f"// GetMetadata 获取{struct_name}的Metadata信息")
        content.append(f"func (s *{struct_name}) GetMetadata() *MetadataV1 {{")
        content.append("\tif s == nil {")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("\treturn s.Metadata")
        content.append("}")
        content.append("")
    
    def _has_field_in_endpoint(self, spec: Dict[str, Any], endpoint_name: str, field_name: str) -> bool:
        """检查link schema的endpoint（src/dest）是否包含指定字段。
        
        如果子schema在properties中重新定义了endpoint，则检查其自身的properties；
        如果endpoint来自link:v1继承（未被覆盖），则检查link:v1的定义。
        """
        spec_prop = spec.get('properties', {}).get('spec', {})
        endpoint_spec = spec_prop.get('properties', {}).get(endpoint_name, {})
        
        if endpoint_spec and isinstance(endpoint_spec, dict):
            # 子schema显式定义了endpoint，检查其properties
            endpoint_props = endpoint_spec.get('properties', {})
            if endpoint_props:
                return field_name in endpoint_props
        
        # 没有显式定义endpoint，检查是否从link:v1继承
        extends = spec_prop.get('extends', [])
        if isinstance(extends, list):
            for extend_ref in extends:
                parent_def = self._resolve_type_ref(extend_ref)
                if parent_def and 'spec' in parent_def:
                    parent_props = parent_def['spec'].get('properties', {})
                    endpoint_parent = parent_props.get(endpoint_name, {})
                    if isinstance(endpoint_parent, dict):
                        return field_name in endpoint_parent.get('properties', {})
        
        return False

    def _generate_link_object_methods(self, content: List[str], struct_name: str, spec: Dict[str, Any]):
        """生成UModelLinkObject接口方法"""
        src_has_filter = self._has_field_in_endpoint(spec, 'src', 'filter')
        dest_has_filter = self._has_field_in_endpoint(spec, 'dest', 'filter')
        
        # 生成GetSrc方法
        content.append(f"// GetSrc 获取{struct_name}的源端点信息")
        content.append(f"func (s *{struct_name}) GetSrc() *LinkEndpoint {{")
        content.append("\tif s == nil || s.Spec == nil {")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("\t// 检查Spec是否有Src字段")
        content.append("\tif s.Spec.Src != nil {")
        content.append("\t\treturn &LinkEndpoint{")
        content.append("\t\t\tDomain: s.Spec.Src.Domain,")
        content.append("\t\t\tKind:   s.Spec.Src.Kind,")
        content.append("\t\t\tName:   s.Spec.Src.Name,")
        if src_has_filter:
            content.append("\t\t\tFilter: s.Spec.Src.Filter,")
        content.append("\t\t}")
        content.append("\t}")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        # 生成GetDest方法
        content.append(f"// GetDest 获取{struct_name}的目标端点信息")
        content.append(f"func (s *{struct_name}) GetDest() *LinkEndpoint {{")
        content.append("\tif s == nil || s.Spec == nil {")
        content.append("\t\treturn nil")
        content.append("\t}")
        content.append("\t// 检查Spec是否有Dest字段")
        content.append("\tif s.Spec.Dest != nil {")
        content.append("\t\treturn &LinkEndpoint{")
        content.append("\t\t\tDomain: s.Spec.Dest.Domain,")
        content.append("\t\t\tKind:   s.Spec.Dest.Kind,")
        content.append("\t\t\tName:   s.Spec.Dest.Name,")
        if dest_has_filter:
            content.append("\t\t\tFilter: s.Spec.Dest.Filter,")
        content.append("\t\t}")
        content.append("\t}")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
    
    def _generate_main_package(self):
        """生成主包文件"""
        content = []
        content.append("// Code generated by schema_go_generator_v2.py. DO NOT EDIT.")
        content.append("")
        content.append("// Package umodel provides Go SDK for UModel Schema")
        content.append("package umodel")
        content.append("")
        content.append("import (")
        content.append('\t"encoding/json"')
        content.append('\t"fmt"')
        content.append('\t"io"')
        content.append('\t"strings"')
        content.append('\t"gopkg.in/yaml.v3"')
        content.append(")")
        content.append("")
        content.append("// Version SDK版本")
        content.append('const Version = "2.0.0"')
        content.append("")
        
        # 添加解析辅助函数
        content.append("// ParseJSON 从JSON数据解析UModel对象")
        content.append("func ParseJSON(data []byte, v interface{}) error {")
        content.append("\treturn json.Unmarshal(data, v)")
        content.append("}")
        content.append("")
        
        content.append("// ParseYAML 从YAML数据解析UModel对象")
        content.append("func ParseYAML(data []byte, v interface{}) error {")
        content.append("\treturn yaml.Unmarshal(data, v)")
        content.append("}")
        content.append("")
        
        # 添加类型注册表
        content.append("// TypeRegistry 类型注册表，用于动态创建类型实例")
        content.append("var TypeRegistry = map[string]func() interface{}{")
        
        # 注册所有生成的类型
        for type_key, go_type_name in sorted(self.go_type_mapping.items()):
            if self.type_registry.get(type_key, {}).get('file') != 'core':
                continue
            content.append(f'\t"{type_key}": func() interface{{ }} {{ return &{go_type_name}{{}} }},')
        
        content.append("}")
        content.append("")
        
        # 添加ParseType函数
        content.append("// ParseType 根据类型名称解析对象")
        content.append("func ParseType(typeName string, data []byte, format string) (interface{}, error) {")
        content.append("\tfactory, ok := TypeRegistry[typeName]")
        content.append("\tif !ok {")
        content.append('\t\treturn nil, fmt.Errorf("unknown type: %s", typeName)')
        content.append("\t}")
        content.append("")
        content.append("\tobj := factory()")
        content.append('\tswitch format {')
        content.append('\tcase "json":')
        content.append("\t\terr := json.Unmarshal(data, obj)")
        content.append("\t\treturn obj, err")
        content.append('\tcase "yaml":')
        content.append("\t\terr := yaml.Unmarshal(data, obj)")
        content.append("\t\treturn obj, err")
        content.append("\tdefault:")
        content.append('\t\treturn nil, fmt.Errorf("unsupported format: %s", format)')
        content.append("\t}")
        content.append("}")
        content.append("")
        
        # 添加预解析结构体
        content.append("// umodelHeader 用于预解析UModel对象的kind和schema信息，并进行基础校验")
        content.append("type umodelHeader struct {")
        content.append('\tKind     string `json:"kind" yaml:"kind"`')
        content.append('\tMetadata struct {')
        content.append('\t\tDomain string `json:"domain" yaml:"domain"`')
        content.append('\t\tName   string `json:"name" yaml:"name"`')
        content.append('\t} `json:"metadata" yaml:"metadata"`')
        content.append('\tSchema struct {')
        content.append('\t\tVersion string `json:"version" yaml:"version"`')
        content.append('\t} `json:"schema" yaml:"schema"`')
        content.append('\tSpec map[string]interface{} `json:"spec" yaml:"spec"`')
        content.append("}")
        content.append("")
        
        # 添加Valid方法
        content.append("// Valid 验证UModel对象的基础结构是否符合要求")
        content.append("func (h *umodelHeader) Valid() error {")
        content.append("\t// 检查必要字段是否存在且非空")
        content.append('\tif h.Kind == "" {')
        content.append('\t\treturn fmt.Errorf("missing required field: kind")')
        content.append("\t}")
        content.append("")
        content.append('\tif h.Schema.Version == "" {')
        content.append('\t\treturn fmt.Errorf("missing required field: schema.version")')
        content.append("\t}")
        content.append("")
        content.append('\tif h.Metadata.Domain == "" {')
        content.append('\t\treturn fmt.Errorf("missing required field: metadata.domain")')
        content.append("\t}")
        content.append("")
        content.append('\tif h.Metadata.Name == "" {')
        content.append('\t\treturn fmt.Errorf("missing required field: metadata.name")')
        content.append("\t}")
        content.append("")
        content.append('\tif h.Spec == nil {')
        content.append('\t\treturn fmt.Errorf("missing required field: spec")')
        content.append("\t}")
        content.append("")
        content.append('\tif len(h.Spec) == 0 {')
        content.append('\t\treturn fmt.Errorf("spec field cannot be empty")')
        content.append("\t}")
        content.append("")
        content.append("\t// spec 必须是非空的 JSON Object")
        content.append("")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        # 添加ParseJsonUModel函数
        content.append("// ParseJsonUModel 从JSON数据自动解析UModel对象")
        content.append("func ParseJsonUModel(data []byte) (UModelCoreObject, error) {")
        content.append("\t// 先解析header获取基础信息")
        content.append("\tvar header umodelHeader")
        content.append("\tif err := json.Unmarshal(data, &header); err != nil {")
        content.append('\t\treturn nil, fmt.Errorf("failed to parse header: %w", err)')
        content.append("\t}")
        content.append("")
        content.append("\t// 验证UModel基础结构")
        content.append("\tif err := header.Valid(); err != nil {")
        content.append("\t\treturn nil, err")
        content.append("\t}")
        content.append("")
        content.append("\t// 构建类型键")
        content.append('\ttypeKey := fmt.Sprintf("%s:%s", header.Kind, header.Schema.Version)')
        content.append("")
        content.append("\t// 查找类型工厂")
        content.append("\tfactory, ok := TypeRegistry[typeKey]")
        content.append("\tif !ok {")
        content.append("\t\t// 尝试查找兼容版本")
        content.append("\t\t// 如果版本是v0.x.x，尝试v1.0.0")
        content.append('\t\tif strings.HasPrefix(header.Schema.Version, "v0.") {')
        content.append('\t\t\taltTypeKey := fmt.Sprintf("%s:v1.0.0", header.Kind)')
        content.append("\t\t\tfactory, ok = TypeRegistry[altTypeKey]")
        content.append("\t\t}")
        content.append("\t\tif !ok {")
        content.append('\t\t\treturn nil, fmt.Errorf("unknown type: %s (tried %s)", typeKey, typeKey)')
        content.append("\t\t}")
        content.append("\t}")
        content.append("")
        content.append("\t// 创建实例并解析")
        content.append("\tobj := factory()")
        content.append("\tif err := json.Unmarshal(data, obj); err != nil {")
        content.append('\t\treturn nil, fmt.Errorf("failed to parse object: %w", err)')
        content.append("\t}")
        content.append("")
        content.append("\t// 转换为UModelCoreObject")
        content.append("\tcoreObj, ok := obj.(UModelCoreObject)")
        content.append("\tif !ok {")
        content.append('\t\treturn nil, fmt.Errorf("object does not implement UModelCoreObject: %s", typeKey)')
        content.append("\t}")
        content.append("")
        content.append("\treturn coreObj, nil")
        content.append("}")
        content.append("")
        
        # 添加ParseYamlUModel函数
        content.append("// ParseYamlUModel 从YAML数据自动解析UModel对象")
        content.append("func ParseYamlUModel(data []byte) (UModelCoreObject, error) {")
        content.append("\t// 先解析header获取基础信息")
        content.append("\tvar header umodelHeader")
        content.append("\tif err := yaml.Unmarshal(data, &header); err != nil {")
        content.append('\t\treturn nil, fmt.Errorf("failed to parse header: %w", err)')
        content.append("\t}")
        content.append("")
        content.append("\t// 验证UModel基础结构")
        content.append("\tif err := header.Valid(); err != nil {")
        content.append("\t\treturn nil, err")
        content.append("\t}")
        content.append("")
        content.append("\t// 构建类型键")
        content.append('\ttypeKey := fmt.Sprintf("%s:%s", header.Kind, header.Schema.Version)')
        content.append("")
        content.append("\t// 查找类型工厂")
        content.append("\tfactory, ok := TypeRegistry[typeKey]")
        content.append("\tif !ok {")
        content.append("\t\t// 尝试查找兼容版本")
        content.append("\t\t// 如果版本是v0.x.x，尝试v1.0.0")
        content.append('\t\tif strings.HasPrefix(header.Schema.Version, "v0.") {')
        content.append('\t\t\taltTypeKey := fmt.Sprintf("%s:v1.0.0", header.Kind)')
        content.append("\t\t\tfactory, ok = TypeRegistry[altTypeKey]")
        content.append("\t\t}")
        content.append("\t\tif !ok {")
        content.append('\t\t\treturn nil, fmt.Errorf("unknown type: %s (tried %s)", typeKey, typeKey)')
        content.append("\t\t}")
        content.append("\t}")
        content.append("")
        content.append("\t// 创建实例并解析")
        content.append("\tobj := factory()")
        content.append("\tif err := yaml.Unmarshal(data, obj); err != nil {")
        content.append('\t\treturn nil, fmt.Errorf("failed to parse object: %w", err)')
        content.append("\t}")
        content.append("")
        content.append("\t// 转换为UModelCoreObject")
        content.append("\tcoreObj, ok := obj.(UModelCoreObject)")
        content.append("\tif !ok {")
        content.append('\t\treturn nil, fmt.Errorf("object does not implement UModelCoreObject: %s", typeKey)')
        content.append("\t}")
        content.append("")
        content.append("\treturn coreObj, nil")
        content.append("}")
        content.append("")
        
        # 添加辅助函数
        content.append("// IsCoreObject 判断对象是否实现了UModelCoreObject接口")
        content.append("func IsCoreObject(obj interface{}) bool {")
        content.append("\t_, ok := obj.(UModelCoreObject)")
        content.append("\treturn ok")
        content.append("}")
        content.append("")
        
        content.append("// IsLinkObject 判断对象是否实现了UModelLinkObject接口")
        content.append("func IsLinkObject(obj interface{}) bool {")
        content.append("\t_, ok := obj.(UModelLinkObject)")
        content.append("\treturn ok")
        content.append("}")
        content.append("")
        
        content.append("// GetObjectMetadata 获取任意UModel对象的Metadata（如果支持）")
        content.append("func GetObjectMetadata(obj interface{}) *MetadataV1 {")
        content.append("\tif coreObj, ok := obj.(UModelCoreObject); ok {")
        content.append("\t\treturn coreObj.GetMetadata()")
        content.append("\t}")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        content.append("// GetObjectSchema 获取任意UModel对象的Schema（如果支持）")
        content.append("func GetObjectSchema(obj interface{}) *SchemaV1 {")
        content.append("\tif coreObj, ok := obj.(UModelCoreObject); ok {")
        content.append("\t\treturn coreObj.GetSchema()")
        content.append("\t}")
        content.append("\treturn nil")
        content.append("}")
        content.append("")
        
        content.append("// GetLinkEndpoints 获取Link对象的源和目标端点（如果支持）")
        content.append("func GetLinkEndpoints(obj interface{}) (src, dest *LinkEndpoint) {")
        content.append("\tif linkObj, ok := obj.(UModelLinkObject); ok {")
        content.append("\t\treturn linkObj.GetSrc(), linkObj.GetDest()")
        content.append("\t}")
        content.append("\treturn nil, nil")
        content.append("}")
        content.append("")
        
        # 添加init函数以避免unused import错误
        content.append("// init 确保所有导入的包都被使用")
        content.append("func init() {")
        content.append("\t_ = io.Discard")
        content.append("\t_ = fmt.Sprintf")
        content.append("}")
        content.append("")
        
        # 写入文件
        output_file = self.output_dir / "umodel.go"
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(content))
        
        print(f"✅ 生成主包文件: {output_file.name}")


def main():
    """主函数"""
    print("🚀 UModel Schema Go SDK Generator V2 启动")
    print("=" * 50)
    
    # 配置路径
    schemas_dir = "schemas"
    output_dir = "sdk/go/umodel"
    
    # 检查必要的目录
    if not os.path.exists(schemas_dir):
        print(f"❌ schemas目录不存在: {schemas_dir}")
        return
    
    # 创建生成器
    generator = GoCodeGeneratorV2(schemas_dir, output_dir)
    
    try:
        # 生成所有代码
        generator.generate_all()
        
        print(f"\n✅ Go SDK V2代码已生成到: {output_dir}")
        print("\n📝 使用说明:")
        print("1. 将生成的代码复制到您的Go项目中")
        print("2. 安装依赖: go get gopkg.in/yaml.v3")
        print("3. 导入包: import \"your-module/umodel\"")
        print("\n🎯 V2版本特性:")
        print("- 使用Go的嵌入特性实现继承关系")
        print("- 生成更简洁、高复用的代码")
        print("- 保留原始schema的结构关系")
        print("- 支持动态类型创建和解析")
        print("- 提供通用接口UModelCoreObject和UModelLinkObject")
        
        print("\n📚 接口使用示例:")
        print("// 判断对象类型")
        print("if IsCoreObject(obj) {")
        print("    metadata := GetObjectMetadata(obj)")
        print("    schema := GetObjectSchema(obj)")
        print("    kind := obj.GetKind() // 获取对象类型")
        print("}")
        print("")
        print("// 处理Link对象")
        print("if IsLinkObject(obj) {")
        print("    src, dest := GetLinkEndpoints(obj)")
        print("    fmt.Printf(\"Link from %s to %s\\n\", src.Name, dest.Name)")
        print("}")
        print("")
        print("// 自动解析YAML文件")
        print("data, _ := os.ReadFile(\"sls_front_metric.yaml\")")
        print("obj, err := ParseYamlUModel(data)")
        print("if err == nil {")
        print("    fmt.Printf(\"Parsed object kind: %s\\n\", obj.GetKind())")
        print("    metadata := obj.GetMetadata()")
        print("    fmt.Printf(\"Object name: %s\\n\", metadata.Name)")
        print("}")
        print("")
        print("// 自动解析JSON数据")
        print("jsonData := []byte(`{\"kind\":\"metric_set\",\"schema\":{\"version\":\"v1.0.0\"}}`)")
        print("obj, err = ParseJsonUModel(jsonData)")
        print("if err == nil {")
        print("    // obj 自动被解析为 MetricSetV100 类型")
        print("    if metricSet, ok := obj.(*MetricSetV100); ok {")
        print("        // 使用具体类型的方法")
        print("        fmt.Printf(\"MetricSet: %s\\n\", metricSet.Metadata.Name)")
        print("    }")
        print("}")
        
    except Exception as e:
        print(f"\n❌ 生成过程中发生错误: {e}")
        import traceback
        traceback.print_exc()


if __name__ == "__main__":
    main() 
