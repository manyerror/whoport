package cli

import (
	"reflect"
	"testing"
)

func TestParsePortSpecs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []uint16
	}{
		{"单个端口", []string{"3000"}, []uint16{3000}},
		{"多个端口参数", []string{"3000", "8080"}, []uint16{3000, 8080}},
		{"逗号分隔", []string{"3000,8080"}, []uint16{3000, 8080}},
		{"端口段", []string{"3000-3002"}, []uint16{3000, 3001, 3002}},
		{"混合写法", []string{"3000,9000-9001"}, []uint16{3000, 9000, 9001}},
		{"去重并排序", []string{"8080", "3000", "8080"}, []uint16{3000, 8080}},
		{"单元素段", []string{"3000-3000"}, []uint16{3000}},
		{"上界 65535", []string{"65535"}, []uint16{65535}},
		{"含 65535 的段不溢出", []string{"65534-65535"}, []uint16{65534, 65535}},
		{"空白容忍", []string{" 3000 , 8080 "}, []uint16{3000, 8080}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parsePortSpecs(c.in)
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parsePortSpecs(%v) = %v, 期望 %v", c.in, got, c.want)
			}
		})
	}
}

func TestParsePortSpecsErrors(t *testing.T) {
	cases := []struct {
		name string
		in   []string
	}{
		{"端口越界", []string{"99999"}},
		{"零端口", []string{"0"}},
		{"非数字", []string{"abc"}},
		{"空段", []string{"3000-"}},
		{"倒序段", []string{"3010-3000"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parsePortSpecs(c.in); err == nil {
				t.Errorf("parsePortSpecs(%v) 应当报错，却通过了", c.in)
			}
		})
	}
}
