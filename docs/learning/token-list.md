# 接続一覧

仕様は [07 の List](../specs/07-agent-connection.md)。画面は [接続画面](connection-screen.md)。本人の接続トークンを新しい順で返す。再接続が完了していない置き換え先は、行ごと出さない。再接続の状態は変えない。開始は [再接続の開始](reconnect-start.md)。期限内の未完了が学習記録APIを通る道は [再接続の確認](reconnect-confirm.md)。

## 何のためか

使う人は、Webにログインした本人。今ある接続を見る。
入力はユーザーID。出力は `id`、`name`、`created_at`、`expires_at`、`revoked_at`。行は更新しない。
`state` が `completed` でない再接続リクエストの `new_token_id` は返さない。24時間以内の `pending` と `confirmed` も、期限を過ぎたままの `pending` と `confirmed` も、`superseded` と `expired` も出さない。

## 状態の流れ

この機能は再接続の状態を変えない。出すか外すかだけを分ける。

| トークン | 一覧 |
|---|---|
| 通常のトークン。再接続の置き換え先ではない | 出す。期限切れも失効済みも出す |
| 旧トークン。再接続の途中でも、完了で失効したあとでも | 出す。`new_token_id` ではない |
| `state` が `completed` の置き換え先 | 出す。使えるかは `expires_at` と `revoked_at`。期限切れ・失効済みも、通常のトークンとして出す |
| `pending`、`confirmed`、`superseded`、`expired` の置き換え先 | 出さない。24時間以内の `pending` と `confirmed` も含む |

有効フラグは無い。残った行の失効と期限は、列の値で見る。

## 呼び出しの流れ

`GET /connection-tokens`。呼び元は [handler.go RegisterRoutes](../../backend/internal/connectiontoken/handler.go)。WebログインとCSRFのあと。`Cache-Control: no-store`。Bearerだけでは届かない。

| 順 | 呼び出し | SQL |
|---|---|---|
| 1 | [handler.go Handler.List](../../backend/internal/connectiontoken/handler.go) | なし。未ログインは401 |
| 2 | [manage.go Service.List](../../backend/internal/connectiontoken/manage.go) | 本人の `connection_tokens` を新しい順で読む。下の `NOT EXISTS` |
| 3 | [service.go scanToken](../../backend/internal/connectiontoken/service.go) | なし。各行を `Token` にする |
| 4 | `Handler.List` が `publicToken` | なし。`id`、`name`、`created_at`、`expires_at`、`revoked_at`。所有者もハッシュも再接続の状態も出さない |

```go
query := `
    SELECT id, user_id, name, created_at, expires_at, revoked_at
    FROM connection_tokens
    WHERE user_id=$1
    AND NOT EXISTS (
        SELECT 1 FROM connection_reconnections r
        WHERE r.new_token_id=connection_tokens.id AND r.state <> 'completed'
    )
    ORDER BY created_at DESC, id DESC
`
rows, err := s.db.QueryContext(ctx, query, userID)
for rows.Next() {
    token, err := scanToken(rows)
}
```

`QueryContext` が問い合わせと、引数のユーザーIDを実行する。`scanToken` が各行の `Scan` を呼ぶ。[NOT EXISTS](glossary.md#not-exists) は、内側が1行も無いトークンだけを残す。

認証はこの問い合わせを使わない。期限内の `pending` と `confirmed` は、一覧に出なくても学習記録APIでは通る。失効もこの問い合わせを使わない。一覧に出ないトークンも、`Revoke` はそのIDを失効できる。失効しても一覧には戻さない。`Revoke` は `revoked_at` が空のときだけ今を入れる。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| [handler.go RegisterRoutes](../../backend/internal/connectiontoken/handler.go) | `GET /connection-tokens` を、WebログインとCSRFの後ろに載せる | mux、認証、信頼するOrigin | 登録エラーか nil | なし |
| [handler.go Handler.List](../../backend/internal/connectiontoken/handler.go) | 一覧のHTTP入口 | WebログインのHTTP | 200の配列。0件は `[]` | なし。再接続の状態は足さない |
| [manage.go List](../../backend/internal/connectiontoken/manage.go) | 本人の接続を新しい順で返す。未完了の置き換え先は除く | ユーザーID | `[]Token` | なし。`state` が `completed` でない `new_token_id` を外す |
| [service.go scanToken](../../backend/internal/connectiontoken/service.go) | トークンの1行を構造体へ写す | `Scan` できる行 | `Token` | なし |
| [manage.go Revoke](../../backend/internal/connectiontoken/manage.go) | 本人のトークンを失効する。一覧の条件は見ない | ユーザーID、トークンID | errorか nil | `revoked_at` が空のときだけ今。一覧の除外とは別の文 |

`Handler.List` は、`manage.go List` の結果をJSONにするだけ。未完了の置き換え先は、行ごと返さない。件数を数える呼び出し側は、そのぶん少なくなる。

## なぜそうしたか

| 選択 | 理由 |
|---|---|
| 未完了の置き換え先を出さない | 作りかけの鍵は、始めて10時間でも、24時間を過ぎても、やり直しで捨てたあとでも、接続の一覧に混ぜない |
| 24時間超過のまま退役していない行も外す | その置き換え先は、失効日時が空で期限が約90日後のまま残ることがある。状態で外す前の一覧では、有効な鍵に見えた。`GetReconnect` は同じ状況を、DBを変えずに `expired` と返す。一覧は行を返さないので、その見え方の差は起きない |
| 完了した置き換え先は残す | 置き換えが終わった接続は、通常の鍵と同じく期限と失効で出す。期限切れ・失効済みを隠す条件にはしない |
| 旧トークンは残す | 再接続の途中でも、どれを差し替えているかを一覧から消さない。除外のキーは `new_token_id` |
| 認証の条件は変えない | 認証は期限内の `pending` と `confirmed` を通す。一覧の `NOT EXISTS` は `state <> 'completed'` 全部。確認の通り道を、接続の一覧の形にはしない |
| 失効は一覧の条件を見ない | 出していないトークンも、IDを知っていれば失効できる。失効が一覧へ戻す副作用は無い |

状態取得の CASE と、この一覧は別の読み方。状態取得は [再接続の状態取得](reconnect-status.md)。退役して `revoked_at` が入ったあとも、`expired` と `superseded` の置き換え先は、この一覧には出ない。

## 未決

なし。PR #29 のレビューで未決だった一覧の見せ方は、`state` が `completed` でない `new_token_id` を出さないことで決まっている。仕様07の `List` に記録済み。画面は [接続画面](connection-screen.md)。
