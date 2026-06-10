// resource_reader.go 提供 resource feature 到 enrichextract 的适配层。
package enrichextract

import "kgbrain/internal/resource"

type resourceReaderAdapter struct {
	service *resource.Service
}

// NewResourceReader 将 resource feature 适配为结构化抽取所需的最小读取接口。
func NewResourceReader(service *resource.Service) ResourceReader {
	return &resourceReaderAdapter{service: service}
}

func (r *resourceReaderAdapter) GetLLM(id string) (*resource.LLMResource, error) {
	res, err := r.service.GetLLM(id)
	if err != nil {
		if resource.IsNotFound(err) {
			return nil, &notFoundError{message: "llm resource not found"}
		}
		return nil, err
	}
	return res, nil
}

func (r *resourceReaderAdapter) GetDatabase(id string) (*resource.DatabaseResource, error) {
	res, err := r.service.GetDatabase(id)
	if err != nil {
		if resource.IsNotFound(err) {
			return nil, &notFoundError{message: "database resource not found"}
		}
		return nil, err
	}
	return res, nil
}
