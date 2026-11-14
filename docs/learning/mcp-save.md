# 学習記録の保存ツール

仕様は [08](../specs/08-local-mcp.md)。一覧は [学習記録の一覧ツール](mcp-list.md)。詳細取得はまだ無い。

## 何のためか

使う人は、手元のAIツール。入力は `request_id`、`theme`、`questions`。出力は保存済みの `id`、`request_id`、`theme`、`questions`、`created_at`。新しいサーバーAPIは作らない。

## 状態の流れ

状態は無い。呼ぶたびに `POST /learning-records` を1回送る。

## 呼び出しの流れ

stdio。待ち受けるポートは無い。登録は [server.go New](../../mcp/internal/server/server.go)。

| 順 | 呼び出し | SQL |
|---|---|---|
| 1 | [server.go saveLearningRecord](../../mcp/internal/server/server.go) | なし |
| 2 | [client.go DoJSON](../../mcp/internal/api/client.go) | なし。POST。Bearer |

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `saveLearningRecord` | 確認済みの記録を保存する | context、client、`saveInput` | 保存結果かツールエラー | なし。保存はAPI側 |

`saveInput` のJSONは `learningrecord.SaveInput` と同じ。`hint_used` は必須なので、JSONから省かない。

## なぜそうしたか

失敗は一覧と同じ `failureText`。ステータスと `error` のコードだけを返し、応答本文とトークンは返さない。仕様08は `validation_failed` の `details` も返すと書く。この段階ではコードだけにする。

## 未決

詳細取得ツールは未実装。Issue #44 と全体ブロック2は未完了。
