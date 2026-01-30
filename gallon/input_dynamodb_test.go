package gallon

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestInputPluginDynamoDbConfigSchemaColumn_getValue_NullNumber(t *testing.T) {
	// Test case: number型のフィールドにNULLが入っている場合
	// 現状: エラーになる
	// 期待: nilを返すべき

	tests := []struct {
		name        string
		columnType  string
		value       types.AttributeValue
		expected    any
		expectError bool
	}{
		{
			name:        "number with valid value",
			columnType:  "number",
			value:       &types.AttributeValueMemberN{Value: "42"},
			expected:    "42",
			expectError: false,
		},
		{
			name:        "number with NULL value",
			columnType:  "number",
			value:       &types.AttributeValueMemberNULL{Value: true},
			expected:    nil,
			expectError: false, // NULL値はnilとして正しく処理される
		},
		{
			name:        "string with valid value",
			columnType:  "string",
			value:       &types.AttributeValueMemberS{Value: "hello"},
			expected:    "hello",
			expectError: false,
		},
		{
			name:        "string with NULL value",
			columnType:  "string",
			value:       &types.AttributeValueMemberNULL{Value: true},
			expected:    nil,
			expectError: false, // NULL値はnilとして正しく処理される
		},
		{
			name:        "boolean with valid value",
			columnType:  "boolean",
			value:       &types.AttributeValueMemberBOOL{Value: true},
			expected:    true,
			expectError: false,
		},
		{
			name:        "boolean with NULL value",
			columnType:  "boolean",
			value:       &types.AttributeValueMemberNULL{Value: true},
			expected:    nil,
			expectError: false, // NULL値はnilとして正しく処理される
		},
		{
			name:        "any with NULL value",
			columnType:  "any",
			value:       &types.AttributeValueMemberNULL{Value: true},
			expected:    nil,
			expectError: false, // anyタイプは現在もNULLをサポートしている
		},
		{
			name:        "any with number value",
			columnType:  "any",
			value:       &types.AttributeValueMemberN{Value: "123"},
			expected:    float64(123),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := InputPluginDynamoDbConfigSchemaColumn{
				Type: tt.columnType,
			}

			result, err := schema.getValue(tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil, result: %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
				}
			}
		})
	}
}

func TestInputPluginDynamoDbConfigSchemaColumn_getValue_NullInNestedObject(t *testing.T) {
	// Test case: object内のnumber型フィールドにNULLが入っている場合
	schema := InputPluginDynamoDbConfigSchemaColumn{
		Type: "object",
		Properties: map[string]InputPluginDynamoDbConfigSchemaColumn{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
	}

	// ageがNULLのオブジェクト
	value := &types.AttributeValueMemberM{
		Value: map[string]types.AttributeValue{
			"name": &types.AttributeValueMemberS{Value: "Alice"},
			"age":  &types.AttributeValueMemberNULL{Value: true},
		},
	}

	result, err := schema.getValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}

	if resultMap["name"] != "Alice" {
		t.Errorf("expected name to be 'Alice', got %v", resultMap["name"])
	}
	if resultMap["age"] != nil {
		t.Errorf("expected age to be nil, got %v", resultMap["age"])
	}
}

func TestInputPluginDynamoDbConfigSchemaColumn_getValue_NullInArray(t *testing.T) {
	// Test case: array内のnumber型アイテムにNULLが入っている場合
	schema := InputPluginDynamoDbConfigSchemaColumn{
		Type: "array",
		Items: &InputPluginDynamoDbConfigSchemaColumn{
			Type: "number",
		},
	}

	// NULLを含む配列
	value := &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberN{Value: "1"},
			&types.AttributeValueMemberNULL{Value: true},
			&types.AttributeValueMemberN{Value: "3"},
		},
	}

	result, err := schema.getValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultSlice, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}

	if len(resultSlice) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(resultSlice))
	}
	if resultSlice[0] != "1" {
		t.Errorf("expected first element to be '1', got %v", resultSlice[0])
	}
	if resultSlice[1] != nil {
		t.Errorf("expected second element to be nil, got %v", resultSlice[1])
	}
	if resultSlice[2] != "3" {
		t.Errorf("expected third element to be '3', got %v", resultSlice[2])
	}
}
