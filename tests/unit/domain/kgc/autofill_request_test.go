package kgc

import (
	"testing"

	domainkgc "kgbrain/internal/domain/kgc"
)

func TestAutofillRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *domainkgc.AutofillRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &domainkgc.AutofillRequest{
				Data:           []map[string]any{{"name": "张角", "birth": "184年"}},
				TargetsExample: []map[string]any{{"product_name": "青花瓷瓶", "dynasty": "明代"}},
			},
			wantErr: false,
		},
		{
			name: "missing data",
			req: &domainkgc.AutofillRequest{
				Data:           []map[string]any{},
				TargetsExample: []map[string]any{{"product_name": "青花瓷瓶"}},
			},
			wantErr: true,
		},
		{
			name: "data exceeds max 100",
			req: &domainkgc.AutofillRequest{
				Data:           make([]map[string]any, 101),
				TargetsExample: []map[string]any{{"product_name": "青花瓷瓶"}},
			},
			wantErr: true,
		},
		{
			name: "missing targets_example",
			req: &domainkgc.AutofillRequest{
				Data:           []map[string]any{{"name": "test"}},
				TargetsExample: []map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "nil targets_example",
			req: &domainkgc.AutofillRequest{
				Data:           []map[string]any{{"name": "test"}},
				TargetsExample: nil,
			},
			wantErr: true,
		},
		{
			name: "targets_example fields inconsistent",
			req: &domainkgc.AutofillRequest{
				Data: []map[string]any{{"name": "test"}},
				TargetsExample: []map[string]any{
					{"product_name": "青花瓷瓶", "dynasty": "明代"},
					{"product_name": "唐三彩马"},
				},
			},
			wantErr: true,
		},
		{
			name: "targets_example fields consistent",
			req: &domainkgc.AutofillRequest{
				Data: []map[string]any{{"name": "test"}},
				TargetsExample: []map[string]any{
					{"product_name": "青花瓷瓶", "dynasty": "明代"},
					{"product_name": "唐三彩马", "dynasty": "唐代"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
