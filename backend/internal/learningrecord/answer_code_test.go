package learningrecord

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDecodeSaveInputAnswerCode(t *testing.T) {
	withCode := validSaveInputForTest(1)
	withCode.Questions[0].Answer.Code = "fmt.Println(1)"
	without := validSaveInputForTest(1)

	encodedWith, err := json.Marshal(withCode)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSaveInput(encodedWith)
	if err != nil {
		t.Fatal(err)
	}
	if got.Questions[0].Answer.Code != "fmt.Println(1)" {
		t.Fatalf("回答コード: %q", got.Questions[0].Answer.Code)
	}

	encodedWithout, err := json.Marshal(without)
	if err != nil {
		t.Fatal(err)
	}
	got, err = DecodeSaveInput(encodedWithout)
	if err != nil {
		t.Fatal(err)
	}
	if got.Questions[0].Answer.Code != "" {
		t.Fatalf("省略時は空: %q", got.Questions[0].Answer.Code)
	}
}

func TestSaveAndGetAnswerCode(t *testing.T) {
	db := newIntegrationDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var owner string
	if err := db.QueryRowContext(ctx, `INSERT INTO users (github_user_id, github_login) VALUES (9, 'answer-code') RETURNING id`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	service := New(db)

	withCode := validSaveInputForTest(1)
	withCode.RequestID = uuid.NewString()
	withCode.Questions[0].Answer.Code = "fmt.Println(1)"
	saved, err := service.Save(ctx, owner, withCode)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, owner, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Questions[0].Answer.Code != "fmt.Println(1)" {
		t.Fatalf("詳細取得: %q", got.Questions[0].Answer.Code)
	}

	without := validSaveInputForTest(1)
	without.RequestID = uuid.NewString()
	saved, err = service.Save(ctx, owner, without)
	if err != nil {
		t.Fatal(err)
	}
	got, err = service.Get(ctx, owner, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Questions[0].Answer.Code != "" {
		t.Fatalf("省略した詳細取得: %q", got.Questions[0].Answer.Code)
	}
}
