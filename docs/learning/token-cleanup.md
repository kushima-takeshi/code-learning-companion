# 定期整理の削除

仕様は [07 の30日保持](../specs/07-agent-connection.md)。

## 何のためか

使えなくなってから30日を過ぎた接続トークンを消す。学習記録とユーザーは残す。
入力は `now`。出力は消した再接続行の件数と、消したトークンの件数。

## 状態の流れ

トークンの状態は変えない。対象の行を消すだけ。

使用不能の起点は、`expires_at` と `revoked_at` の早い方。`state` が `completed` でない再接続の `new_token_id` は、その再接続の `expires_at`（開始から24時間）も起点に数える。旧トークンを消すときは、その旧トークンから出た未完了の置き換え先も一緒に消す。24時間の起点がまだ30日以内でも消す。再接続行だけ消すと、置き換え先が再び使える。`completed` の置き換え先は残す。起点から30日ちょうどは残し、それを過ぎてから消す。

## 呼び出しの流れ

HTTPは無い。`main` がシグナルの context でループを起動する。起動直後は実行せず、24時間後が1回目。

| 順 | 呼び出し | SQL |
|---|---|---|
| 1 | [main.go main](../../backend/cmd/server/main.go) | なし。`RunCleanupSchedule` を goroutine で始める |
| 2 | [cleanup_schedule.go RunCleanupSchedule](../../backend/internal/connectiontoken/cleanup_schedule.go) | なし。`time.NewTicker` で24時間ごとに `CleanupUnusableTokens` |
| 3 | [cleanup.go CleanupUnusableTokens](../../backend/internal/connectiontoken/cleanup.go) | 対象の `connection_tokens` を `FOR UPDATE`。旧トークンの未完了な置き換え先も対象に足す |
| 4 | 同じ関数 | `connection_reconnections` を、`old_token_id` か `new_token_id` が対象なら先に削除 |
| 5 | 同じ関数 | 対象の `connection_tokens` を削除して commit |

```go
cutoff := now.UTC().Truncate(time.Microsecond).Add(-UnusableRetention)
```

`cutoff` より起点が前のトークンだけを消す。30日ちょうど（起点が `cutoff` と同じ）は残る。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `RunCleanupSchedule` | 24時間ごとに削除を呼ぶ | `ctx`, `CleanupSchedule` | なし。キャンセルで戻る | なし。成功と失敗をログする |
| `CleanupUnusableTokens` | 30日を過ぎた使用不能トークンを消す | `ctx`, `now` | `CleanupResult`（再接続行とトークンの件数） | 対象の再接続行、続けて対象のトークン。学習記録とユーザーは触らない |
| `deleteByIDs` | 集めたIDで削除する | `tx`, SQL, ID | 件数 | 渡したSQLの行 |
| `numberedPlaceholders` | `IN` の位置を作る | 個数と開始番号 | `$1,$2,...` | なし |

## なぜそうしたか

外部キーがあるので、再接続行を先に消してからトークンを消す。1つのトランザクションにする。削除は同じ対象を重ねて消さないので、2回目は0件。二重実行の防止はしない。失敗してもすぐには再試行せず、次の24時間に任せる。成功時は削除件数だけをログに出す。

`completed` の置き換え先を24時間で使用不能にしない。完了した接続は、発行から90日の期限のままにする。`pending` のまま24時間を過ぎた置き換え先は、行の状態が `expired` に変わっていなくても、再接続の `expires_at` を起点にする。旧トークンが先に30日を過ぎたときも、未完了の置き換え先は残さない。

## 未決

実サーバーで24時間後の実行は未確認。#46 と全体ブロック2は未完了。画面にトークン履歴は出さない。
