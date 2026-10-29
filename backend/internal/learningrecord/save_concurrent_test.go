package learningrecord

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSaveIntegrationConcurrent(t *testing.T) {
	for _, different := range []bool{false, true} {
		name := "同じ内容"
		if different {
			name = "異なる内容"
		}
		t.Run(name, func(t *testing.T) {
			db := newIntegrationDB(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var ownerID string
			if err := db.QueryRowContext(ctx, `INSERT INTO users (github_user_id, github_login) VALUES (1, 'test-owner') RETURNING id`).Scan(&ownerID); err != nil {
				t.Fatal(err)
			}

			// SELECTは許可し、INSERTだけを待たせる。両方の検索が未保存になる状況を作る。
			blocker, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			if _, err := blocker.ExecContext(ctx, `LOCK TABLE learning_records IN SHARE MODE`); err != nil {
				t.Fatal(err)
			}

			inputA := saveTestInput()
			inputB := inputA
			if different {
				inputB.Theme = "別の学習テーマ"
			}
			type result struct {
				record Record
				err    error
				input  SaveInput
			}
			results := make(chan result, 2)
			service := New(db)
			for _, input := range []SaveInput{inputA, inputB} {
				go func(input SaveInput) {
					record, err := service.Save(ctx, ownerID, input)
					results <- result{record: record, err: err, input: input}
				}(input)
			}

			// 開始時刻の偶然や固定sleepに依存せず、2つのINSERTの待機を実DBで確認する。
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting int
				err := db.QueryRowContext(ctx, `
     SELECT count(*) FROM pg_locks
     WHERE relation = 'learning_records'::regclass
       AND mode = 'RowExclusiveLock' AND NOT granted
    `).Scan(&waiting)
				if err != nil {
					t.Fatalf("INSERT待機の確認に失敗: %v", err)
				}
				if waiting == 2 {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("2つのINSERTが待機する前にタイムアウト")
				case <-ticker.C:
				}
			}
			if err := blocker.Commit(); err != nil {
				t.Fatal(err)
			}

			var winner Record
			successes, conflicts := 0, 0
			received := make([]result, 0, 2)
			for range 2 {
				select {
				case got := <-results:
					received = append(received, got)
				case <-ctx.Done():
					t.Fatal("Saveの完了待ちでタイムアウト")
				}
			}
			for _, got := range received {
				switch {
				case got.err == nil:
					successes++
					if got.record.Theme != got.input.Theme || !reflect.DeepEqual(got.record.Questions, got.input.Questions) {
						t.Fatalf("成功した記録と送信内容が不一致: %+v", got.record)
					}
					if winner.ID != "" && !reflect.DeepEqual(winner, got.record) {
						t.Fatalf("同じ要求なのに異なる保存結果: %+v / %+v", winner, got.record)
					}
					winner = got.record
				case errors.Is(got.err, ErrRequestConflict):
					conflicts++
					if !reflect.DeepEqual(got.record, Record{}) {
						t.Fatalf("競合時は空の記録を期待: %+v", got.record)
					}
				default:
					t.Errorf("想定外の保存エラー: %v", got.err)
				}
			}
			wantSuccesses, wantConflicts := 2, 0
			if different {
				wantSuccesses, wantConflicts = 1, 1
			}
			if successes != wantSuccesses || conflicts != wantConflicts {
				t.Fatalf("成功/競合: got %d/%d, want %d/%d", successes, conflicts, wantSuccesses, wantConflicts)
			}
			reloaded, err := service.Get(ctx, ownerID, winner.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reloaded, winner) {
				t.Fatalf("DBの内容が保存結果と不一致: %+v", reloaded)
			}
			var count int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM learning_records WHERE user_id = $1`, ownerID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("保存件数: got %d, want 1", count)
			}
		})
	}
}
