# 用語集

コードを説明するとき、ここに無い用語で迷ったら1項目足す。説明では、繰り返さずにこのファイルへリンクする。
場所は `ファイル:関数`。SQLの定義だけにあるものは、マイグレーションのファイル名を書く。
main にまだ無いものは、項目の末尾にそう書く。対象は [PR #29](https://github.com/kushima-takeshi/code-learning-companion/pull/29)（ブランチ `codex/connection-reconnect-api`）で確認した。

## Scan と &

`Scan` は、SQLが返した列を、Goの変数へ順に入れる。`&` は、その変数の場所。`Scan` はその場所へ値を書く。

```go
var summary RecordSummary
err := rows.Scan(&summary.ID, &summary.Theme, &summary.CreatedAt)
```

場所: `backend/internal/learningrecord/list.go` の `List`。1行を構造体へ写す例は `backend/internal/connectiontoken/service.go` の `scanToken`。

## QueryRowContext

1行だけ取る問い合わせを実行し、行を返す。列は続けて `Scan` で入れ、0行のときは `sql.ErrNoRows`。複数行は `QueryContext` と `rows.Next`。

```go
query := `
    SELECT id, user_id, request_id, theme, questions, created_at
    FROM learning_records
    WHERE id = $1 AND user_id = $2
`
row := s.db.QueryRowContext(ctx, query, recordID, userID)
err := row.Scan(
    &record.ID,
    &record.UserID,
    &record.RequestID,
    &record.Theme,
    &questionsJSON,
    &record.CreatedAt,
)
```

場所: `backend/internal/learningrecord/query.go` の `Get`。ソースは `QueryRowContext(...).Scan(...)` の1つながり。上は、その3段。

## INSERT … RETURNING

`INSERT` した行のうち、DBが埋めた値を同じ文で返す。学習記録では `id` と `created_at` を `Scan` で受ける。

```go
query := `
    INSERT INTO learning_records (user_id, request_id, theme, questions)
    VALUES ($1, $2, $3, $4)
    RETURNING id, created_at
`
row := s.db.QueryRowContext(ctx, query, userID, input.RequestID, input.Theme, string(questionsJSON))
err = row.Scan(&record.ID, &record.CreatedAt)
```

場所: `backend/internal/learningrecord/repository.go` の `insertRecord`。連携トークンの発行は `backend/internal/connectiontoken/service.go` の `Issue`。

## プレースホルダー（$1）

このリポジトリの PostgreSQL では、`$1` がうしろに並べた1番目の値、`$2` が2番目。SQLの文字列へ値を足して渡す書き方は使わない。

```go
query := `
    SELECT id, theme, created_at
    FROM learning_records
    WHERE user_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2 OFFSET $3
`
rows, err := s.db.QueryContext(ctx, query, userID, limit+1, offset)
```

場所: `backend/internal/learningrecord/list.go` の `List`。`$1` は `userID`、`$2` は `limit+1`、`$3` は `offset`。

## DB と Tx

`*sql.DB` は接続のプールで、問い合わせのたびに1本借りて1文を実行する。`*sql.Tx` は `BeginTx` から `Commit` までのまとまりで、`saveRecords` が途中で return すると `Rollback` で残さない。

```go
// db.go の Open。接続のプール
db, err := sql.Open("pgx", databaseURL)
```

```go
// repository.go の saveRecords。Commit までを1つのまとまりにする
tx, err := s.db.BeginTx(ctx, nil)
err = insertSnapshot(ctx, tx, userID, snapshotID, prepared, keys, files)
err = tx.Commit()
```

場所: プールを作るのは `backend/internal/db/db.go` の `Open`。学習記録は `backend/internal/learningrecord/service.go` の `New` が `db` に持つ。`tx` を作るのは `backend/internal/primport/repository.go` の `saveRecords`。`insertSnapshot` は、その `tx` を受け取る。

## FOR UPDATE

`SELECT ... FOR UPDATE` は、読んだ行を、このトランザクションが終わるまで他の更新から守る。同じ行を先にロックした処理が `Commit` か `Rollback` するまで、次の処理は待つ。

```go
query := `SELECT id,user_id,name,created_at,expires_at,revoked_at FROM connection_tokens WHERE user_id=$1 AND id=$2 FOR UPDATE`
row := tx.QueryRowContext(ctx, query, userID, oldID)
token, err := scanToken(row)
```

場所: `backend/internal/connectiontoken/reconnect.go` の `lockTarget`。同じロックは `lockReconnect` と `activeReplacement`。main にはまだ無い。

## COALESCE / LEAST

`COALESCE(revoked_at, 時刻)` は、左から見て最初の空でない値。`revoked_at` が空なら時刻、すでに入っていればその時刻のまま。
`LEAST(時刻A, 時刻B)` は早い方。`LEAST(COALESCE(revoked_at, 退役時刻), 退役時刻)` は、空なら退役時刻、入っていれば早い方を残す。

```go
// manage.go の Revoke。空のときだけ、今の時刻を入れる
query := `UPDATE connection_tokens SET revoked_at=COALESCE(revoked_at,$3) WHERE id=$1 AND user_id=$2`
result, err := s.db.ExecContext(ctx, query, tokenID, userID, s.now().UTC().Truncate(time.Microsecond))
```

```go
// reconnect.go の retireReconnect。早い方を残す
query := `UPDATE connection_tokens SET revoked_at=LEAST(COALESCE(revoked_at,$2),$2) WHERE id=$1`
_, err := tx.ExecContext(ctx, query, r.NewTokenID, at)
```

場所: `COALESCE` だけは `backend/internal/connectiontoken/manage.go` の `Revoke`。`LEAST` は `backend/internal/connectiontoken/reconnect.go` の `retireReconnect`。`LEAST` のファイルは main にはまだ無い。

## 部分一意インデックス

`WHERE` 付きの `UNIQUE` は、条件に合う行だけを1件にする。`pending`、`confirmed`、`completed` は旧トークンにつき1件で、`superseded` と `expired` は何件でも残る。

```sql
CREATE UNIQUE INDEX connection_reconnections_current_idx
    ON connection_reconnections (old_token_id)
    WHERE state IN ('pending', 'confirmed', 'completed');
```

場所: `backend/migrations/000006_connection_reconnections.up.sql`（関数ではない）。同じ状態の集まりを読むのは `backend/internal/connectiontoken/reconnect.go` の `StartReconnect`。main にはまだ無い。

## 複合外部キー

外部キーが列の組を指す。`(user_id, old_token_id)` は `connection_tokens` の `UNIQUE (user_id, id)` にある組だけを受け付け、`new_token_id` も同じ形。

```sql
FOREIGN KEY (user_id, old_token_id) REFERENCES connection_tokens (user_id, id)
```

場所: `backend/migrations/000006_connection_reconnections.up.sql`（関数ではない）。組を入れるのは `backend/internal/connectiontoken/reconnect.go` の `newReconnect`。main にはまだ無い。

## 関数値（f と f() の違い）

括弧が無い `time.Now` や `s.now` は、関数そのもの。まだ時刻は無い。括弧がある `s.now()` や `clock()` は、その場で関数を呼び、時刻が返る。

```go
// service.go の New。関数をしまっておく
now: time.Now

// service.go の Issue。ここで時刻を読む
createdAt := s.now().UTC().Truncate(time.Microsecond)
```

```go
// ConfirmReconnect。関数を渡す
now, err = activeReplacement(ctx, tx, r, s.now)
```

```go
// activeReplacement。ロックのあとで clock() を呼ぶ
now := clock().UTC().Truncate(time.Microsecond)
```

場所: `backend/internal/connectiontoken/service.go` の `New` と `Issue`。ロックのあとに読む例は `backend/internal/connectiontoken/reconnect.go` の `ConfirmReconnect` と `activeReplacement`。再接続の2関数は main にはまだ無い。

## interface

interface は、メソッドの形だけを決める。その形の型なら `implements` と書かなくても渡せ、`scanToken` は `Scan` を持つ `*sql.Row` も `*sql.Rows` も受ける。

```go
type tokenScanner interface {
	Scan(...any) error
}

token, err := scanToken(row)
```

場所: `backend/internal/connectiontoken/service.go` の `scanToken`。`Issue` は `QueryRowContext` の行を渡し、`backend/internal/connectiontoken/manage.go` の `List` は `rows` を渡す。別の例は `backend/internal/primport/service.go` の `prFetcher`。`backend/internal/primport/prepare.go` の `prepare` が `s.github.FetchPR` を呼ぶ。
