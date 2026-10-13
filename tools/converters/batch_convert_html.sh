#!/bin/bash

# 批量转换 expanded_schemas 中的 YAML 文件为 HTML
# 使用方法: 
#   ./batch_convert_html.sh                    # 生成所有三种版本
#   ./batch_convert_html.sh mixed              # 仅生成混合版本
#   ./batch_convert_html.sh cn                 # 仅生成中文版本  
#   ./batch_convert_html.sh en                 # 仅生成英文版本

set -e  # 遇到错误时退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 参数处理
LANGUAGE_MODE="$1"

# 如果没有指定语言，则生成所有三种版本
if [ -z "$LANGUAGE_MODE" ]; then
    print_info "批量转换 YAML 文件为 HTML（生成所有三种版本）"
    print_info "============================================================"
    
    # 递归调用生成三种版本
    $0 mixed
    $0 cn  
    $0 en
    exit 0
fi

# 验证语言参数
case "$LANGUAGE_MODE" in
    mixed|cn|en)
        ;;
    *)
        print_error "无效的语言参数: $LANGUAGE_MODE"
        print_error "支持的参数: mixed, cn, en"
        exit 1
        ;;
esac

print_info "批量转换 YAML 文件为 HTML ($LANGUAGE_MODE 模式)"
print_info "========================================"

# cd 到项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../../"

# echo 当前目录
print_info "当前目录: $PWD"


# 检查当前目录是否为项目根目录
if [ ! -d "scripts" ] || [ ! -d "expanded_schemas" ]; then
    print_error "请在项目根目录执行此脚本"
    print_error "当前目录: $PWD"
    exit 1
fi

# 检查必要的文件和目录
if [ ! -f "./tools/converters/yaml_to_html.py" ]; then
    print_error "转换脚本不存在: ./tools/converters/yaml_to_html.py"
    exit 1
fi

if [ ! -d "./expanded_schemas" ]; then
    print_error "源目录不存在: ./expanded_schemas"
    exit 1
fi

# 切换到 scripts 目录
print_info "切换到 scripts 目录..."
cd scripts

# 根据语言模式确定输出目录
case "$LANGUAGE_MODE" in
    mixed)
        OUTPUT_DIR="../docs/html"
        ;;
    cn)
        OUTPUT_DIR="../docs/html_cn"
        ;;
    en)
        OUTPUT_DIR="../docs/html_en"
        ;;
esac

# 创建输出目录（如果不存在）
if [ ! -d "$OUTPUT_DIR" ]; then
    print_info "创建输出目录: $OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"
fi

print_info "开始批量转换..."
print_info "源目录: ../expanded_schemas"
print_info "输出目录: $OUTPUT_DIR"
echo

# Prefer project venv python; allow override via PYTHON_BIN env
if [ -z "${PYTHON_BIN:-}" ]; then
    if [ -x "../.venv/bin/python" ]; then
        PYTHON_BIN="../.venv/bin/python"
    else
        PYTHON_BIN="python3"
    fi
fi

if [[ "$PYTHON_BIN" != */* ]]; then
    if ! command -v "$PYTHON_BIN" >/dev/null 2>&1; then
        PYTHON_BIN="python"
    fi
fi

# 统计变量
total_files=0
success_count=0
error_count=0

# 遍历所有 .expanded.yaml 文件
for yaml_file in ../expanded_schemas/*.expanded.yaml; do
    # 检查文件是否存在（处理通配符没有匹配的情况）
    if [ ! -f "$yaml_file" ]; then
        print_warning "没有找到 .expanded.yaml 文件"
        continue
    fi
    
    # 提取文件名（不包含路径和扩展名）
    filename=$(basename "$yaml_file" .expanded.yaml)
    
    # 构造输出文件路径
    html_file="$OUTPUT_DIR/${filename}.html"
    
    print_info "转换: $filename.expanded.yaml -> $filename.html ($LANGUAGE_MODE)"
    
    # 执行转换
    if "$PYTHON_BIN" ../tools/converters/yaml_to_html.py "$yaml_file" -o "$html_file" -l "$LANGUAGE_MODE"; then
        print_success "✓ 转换成功: $filename.html"
        ((++success_count))
    else
        print_error "✗ 转换失败: $filename.expanded.yaml"
        ((++error_count))
    fi
    
    ((++total_files))
    echo
done

# 生成索引文件
print_info "生成索引文件..."
"$PYTHON_BIN" ../tools/converters/generate_index.py -l "$LANGUAGE_MODE" -d "$OUTPUT_DIR"
print_success "索引文件生成完成"

# 返回项目根目录
cd ..


# 输出统计结果
echo "=================================="
print_info "转换完成统计:"
echo "  总文件数: $total_files"
echo "  成功转换: $success_count"
echo "  转换失败: $error_count"

if [ $error_count -eq 0 ]; then
    print_success "🎉 所有文件转换成功！($LANGUAGE_MODE 模式)"
    print_info "HTML文件已生成到: $OUTPUT_DIR"
    exit 0
else
    print_warning "⚠️  有 $error_count 个文件转换失败"
    exit 1
fi 