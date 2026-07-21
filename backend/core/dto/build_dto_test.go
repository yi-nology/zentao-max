package dto

import (
	"testing"
)

func TestBuildQueryDTO_Validate(t *testing.T) {
	tests := []struct {
		name    string
		dto     BuildQueryDTO
		wantErr bool
	}{
		{
			name:    "空参数应该报错",
			dto:     BuildQueryDTO{},
			wantErr: true,
		},
		{
			name:    "仅 ProjectID 有效",
			dto:     BuildQueryDTO{ProjectID: 10},
			wantErr: false,
		},
		{
			name:    "仅 ExecutionID 有效",
			dto:     BuildQueryDTO{ExecutionID: 20},
			wantErr: false,
		},
		{
			name:    "ProjectID 和 ExecutionID 都有",
			dto:     BuildQueryDTO{ProjectID: 10, ExecutionID: 20},
			wantErr: false,
		},
		{
			name:    "ProjectID 为负数应该报错",
			dto:     BuildQueryDTO{ProjectID: -1},
			wantErr: true,
		},
		{
			name:    "ExecutionID 为负数应该报错",
			dto:     BuildQueryDTO{ExecutionID: -5},
			wantErr: true,
		},
		{
			name:    "两者都为负数应该报错",
			dto:     BuildQueryDTO{ProjectID: -1, ExecutionID: -2},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dto.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
