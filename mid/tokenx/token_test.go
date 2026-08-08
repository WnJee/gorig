package tokenx

import (
	"context"
	"testing"
	"time"
)

func TestMemoryTokenExpiryAndEffective(t *testing.T) {
	service := GetDef()
	service.Manager.CleanAll()
	token, err := service.Manager.GenerateAndRecord(context.Background(), "user", map[string]interface{}{"role": "admin"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !service.Manager.IsEffective(token) {
		t.Fatal("new token should be effective")
	}
	time.Sleep(3 * time.Second)
	if service.Manager.IsEffective(token) {
		t.Fatal("expired token should not be effective")
	}
}
