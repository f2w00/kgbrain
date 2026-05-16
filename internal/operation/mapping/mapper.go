package mapping

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"kgbrain/internal/logger"
	"kgbrain/internal/profile"
	"kgbrain/pkg/extract"
	"kgbrain/pkg/hash"

	"go.uber.org/zap"
)

// Mapper 通过 LLM 推导源字段到目标字段的映射关系.
// 结果存入 SQLite mapping_cache 表, 相同字段组合不再请求 LLM.
type Mapper struct {
	// cmCache 缓存 ChatModel 实例, key = hash(baseURL + apiKey + model)
	cmCache     map[string]model.ToolCallingChatModel
	mu          sync.RWMutex
	mappingRepo *MappingRepo
}

func NewMapper(mappingRepo *MappingRepo) *Mapper {
	return &Mapper{
		cmCache:     make(map[string]model.ToolCallingChatModel),
		mappingRepo: mappingRepo,
	}
}

// ExecuteRequest 是 Execute 方法的全部入参.
type ExecuteRequest struct {
	Profile      *profile.Profile
	Example      map[string]any
	TargetFields []string
	Refresh      bool
}

// generateParams 是 generate 内部方法的全部入参.
type generateParams struct {
	cm        model.ToolCallingChatModel
	profileID string
	source    []string
	target    []string
	example   map[string]any
	refresh   bool
}

// Execute 生成字段映射. refresh=true 时忽略缓存, 强制重新生成并覆盖.
func (m *Mapper) Execute(ctx context.Context, req *ExecuteRequest) (*Result, error) {
	source := make([]string, 0, len(req.Example))
	for k := range req.Example {
		source = append(source, k)
	}

	llmCfg, err := req.Profile.ParseLLMConfig()
	if err != nil {
		return nil, fmt.Errorf("parse llm config: %w", err)
	}

	cm, err := m.getOrCreateCM(llmCfg.BaseURL, llmCfg.APIKey, llmCfg.Model)
	if err != nil {
		return nil, err
	}

	return m.generate(ctx, &generateParams{
		cm:        cm,
		profileID: req.Profile.ID,
		source:    source,
		target:    req.TargetFields,
		example:   req.Example,
		refresh:   req.Refresh,
	})
}

// getOrCreateCM 获取或创建 ChatModel, 使用 double-checked locking 避免重复创建.
func (m *Mapper) getOrCreateCM(baseURL, apiKey, model string) (model.ToolCallingChatModel, error) {
	key := llmCacheKey(baseURL, apiKey, model)

	// 首次检查, 不加锁
	m.mu.RLock()
	if cm, ok := m.cmCache[key]; ok {
		m.mu.RUnlock()
		return cm, nil
	}
	m.mu.RUnlock()

	// 二次检查, 加写锁防止并发创建多个实例
	m.mu.Lock()
	defer m.mu.Unlock()

	if cm, ok := m.cmCache[key]; ok {
		return cm, nil
	}

	// temperature=0 降低随机性, 同组字段得到一致结果
	temp := float32(0)
	// response_format=json_object 强制模型输出合法 JSON, 从 API 层面消除非结构化输出
	respFmt := openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONObject,
	}
	cm, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL:        baseURL,
		APIKey:         apiKey,
		Model:          model,
		Temperature:    &temp,
		ResponseFormat: &respFmt,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	m.cmCache[key] = cm
	logger.L().Info("cached new chat model", zap.String("model", model))
	return cm, nil
}

// generate 核心逻辑: 先查 SQLite 缓存 (除非 refresh=true), 未命中则调 LLM 并写入缓存.
func (m *Mapper) generate(ctx context.Context, p *generateParams) (*Result, error) {
	hash := cacheKey(p.source, p.target)

	if !p.refresh {
		row, err := m.mappingRepo.Get(p.profileID, hash)
		if err != nil {
			return nil, fmt.Errorf("get mapping cache: %w", err)
		}
		if row != nil {
			var mapping Mapping
			if err := json.Unmarshal([]byte(row.Mapping), &mapping); err == nil && mapping != nil {
				return &Result{
					Mapping:        mapping,
					UnmappedSource: CalcUnmappedSource(mapping, p.source),
					UnfilledTarget: CalcUnfilledTarget(mapping, p.target),
					Cached:         true,
				}, nil
			}
		}
	}

	prompt := BuildMappingPrompt(p.source, p.target, p.example)
	resp, err := p.cm.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	content := extract.JSON(resp.Content)
	var mapping Mapping
	if err := json.Unmarshal([]byte(content), &mapping); err != nil {
		return nil, fmt.Errorf("parse llm output: %w", err)
	}
	if err := mapping.Validate(p.source, p.target); err != nil {
		return nil, fmt.Errorf("mapping validation: %w", err)
	}

	sourceJSON, _ := json.Marshal(p.source)
	targetJSON, _ := json.Marshal(p.target)
	mappingJSON, _ := json.Marshal(mapping)

	if err := m.mappingRepo.Save(&CacheRow{
		ProfileID:    p.profileID,
		CacheKey:     hash,
		Mapping:      string(mappingJSON),
		SourceFields: string(sourceJSON),
		TargetFields: string(targetJSON),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		logger.L().Warn("save mapping cache failed", zap.Error(err))
	}

	logger.L().Info("mapping generated", zap.Int("mappings", len(mapping)))
	return &Result{
		Mapping:        mapping,
		UnmappedSource: CalcUnmappedSource(mapping, p.source),
		UnfilledTarget: CalcUnfilledTarget(mapping, p.target),
		Cached:         false,
	}, nil
}

// Result 是 mapping.generate 的执行结果.
type Result struct {
	Mapping        Mapping  // 生成的映射关系
	UnmappedSource []string // source 中存在但 mapping 未覆盖的字段
	UnfilledTarget []string // target 中存在但 mapping 未覆盖的字段
	Cached         bool     // 是否来自缓存, false 表示本次由 LLM 生成
}

// cacheKey 基于排序后的 source+target 生成缓存 key, 保证相同字段组合得到一致 key.
func cacheKey(source, target []string) string {
	sort.Strings(source)
	sort.Strings(target)
	return hash.Key(strings.Join(source, ","), strings.Join(target, ","))
}

// llmCacheKey 基于 LLM 连接参数生成 ChatModel 缓存 key.
func llmCacheKey(baseURL, apiKey, model string) string {
	return hash.Key(baseURL, apiKey, model)
}
