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
			expectError: true, // 現在の実装ではエラーになる（修正後はfalseになるべき）
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
			expectError: true, // 現在の実装ではエラーになる（修正後はfalseになるべき）
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
			expectError: true, // 現在の実装ではエラーになる（修正後はfalseになるべき）
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
	if err == nil {
		t.Logf("Result: %v", result)
		t.Log("Note: This test currently expects error. After fix, it should return {name: Alice, age: nil}")
	} else {
		t.Logf("Got expected error (current behavior): %v", err)
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
	if err == nil {
		t.Logf("Result: %v", result)
		t.Log("Note: This test currently expects error. After fix, it should return [1, nil, 3]")
	} else {
		t.Logf("Got expected error (current behavior): %v", err)
	}
}
