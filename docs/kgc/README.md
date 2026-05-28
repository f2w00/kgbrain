# kgc 领域文档

> kgc（Knowledge Graph Construction）领域相关设计与实现文档。

## 目录结构

```
kgc/
├── README.md              # 本文档，领域文档索引
├── enrich/                # 数据补全（enrich）相关
│   └── 001-kgc-enrich-image-optimization.md  # 图片优化方案
└── autofill/             # 智能转换（autofill）相关
    └── 001-autofill-设计文档.md              # autofill 功能设计
```

## 功能列表

| 功能 | 描述 | 文档 |
|------|------|------|
| kgc.enrich | 多模态数据补全（文本+图片），需定义 tasks | [enrich/](enrich/) |
| kgc.autofill | 源结构 → 目标结构智能转换，返回置信度 | [autofill/](autofill/) |

## 代码位置

| 层级 | 路径 |
|------|------|
| domain | `internal/domain/kgc/` |
| application | `internal/application/kgc/` |
| delivery | `internal/delivery/rpc/handlers/kgc.go` |
