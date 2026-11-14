# 学習記録の詳細取得ツール

仕様は [08](../specs/08-local-mcp.md)。一覧は [学習記録の一覧ツール](mcp-list.md)。保存は [学習記録の保存ツール](mcp-save.md)。

## 何のためか

使う人は、手元のAIツール。入力は `id`。出力は保存済みの `id`、`request_id`、`theme`、`questions`、`created_at`。新しいサーバーAPIは作らない。

## 状態の流れ

状態は無い。呼ぶたびに `GET /learning-records/{id}` を1回送る。

## 呼び出しの流れ

stdio。待ち受けるポートは無い。登録は [server.go New](../../mcp/internal/server/server.go)。

| 順 | 呼び出し | SQL |
|---|---|---|
| 1 | [server.go getLearningRecord](../../mcp/internal/server/server.go) | なし。`id` はパスへエスケープする |
| 2 | [client.go DoJSON](../../mcp/internal/api/client.go) | なし。GET。Bearer |

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `getLearningRecord` | 1件の中身を取る | context、client、`getInput` | 記録かツールエラー | なし |

## なぜそうしたか

失敗は一覧・保存と同じ `failureText`。ステータスと `error` のコードだけを返し、応答本文とトークンは返さない。`id` に `/` があっても、パスの区切りにならない。

## 未決

実プロセスでの確認は未了。Issue #44 と全体ブロック2は未完了。
