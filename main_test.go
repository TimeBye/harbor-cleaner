package main

import (
	"testing"
)

func Test_generateRegexByKeys(t *testing.T) {
	type args struct {
		keys string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"empty", args{""}, ":"},
		{",", args{","}, ":"},
		{",,", args{",,"}, ":"},
		{",,,", args{",,,"}, ":"},
		{"dev", args{"dev"}, ".*dev.*"},
		{"dev,test", args{"dev,test"}, ".*dev.*|.*test.*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateRegexByKeys(tt.args.keys); got != tt.want {
				t.Errorf("generateRegexByKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}