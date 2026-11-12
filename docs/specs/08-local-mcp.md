# 08. ローカルMCPの契約案

更新日: 2026-11-12
状態: 案。合意前であり、実装しない。全体ブロック2は未完了。

この文書は、Issue [#44](https://github.com/kushima-takeshi/code-learning-companion/issues/44) の最初の契約だけを書く。薄いローカルアダプターが、既存の学習記録APIを連携トークンで呼ぶ。サーバーのAPIは増やさない。ツールは一覧、保存、詳細取得の3つ。詳細取得は既存の `GET /learning-records/{id}` を使う。

呼び先の契約は [仕様03](03-learning-questions.md)。認証は [仕様07](07-agent-connection.md)。接続方式の案は [第二案](../implementation-plan-v2.md) の「MCP接続とGoサーバー」。

## 作らないもの

- アダプターの実装、配布、実プロセスでの起動確認。
- 1つのAIツールからの保存・再取得（[Issue #45](https://github.com/kushima-takeshi/code-learning-companion/issues/45)）。
- 使用不能トークンの定期整理（[Issue #46](https://github.com/kushima-takeshi/code-learning-companion/issues/46)）。
- 新しいサーバーAPI、再接続画面、スキル、出題。

## トークンの渡し方

原本は、アダプターの起動設定に置く。ツールの引数、スキル本文、会話、学習記録のJSONには入れない。Webの `session` Cookie は使わない。

呼び出しは `Authorization: Bearer <原本>` の1つだけ。サーバーは Bearer から所有者を決める。入力に所有者は無い。

連携トークンは `MCP_CONNECTION_TOKEN`、APIの基点は `MCP_API_BASE_URL` で確定（2026-11-12）。基点が未設定のときは `http://localhost:8080`。`VITE_` は画面のビルドに入るので、トークンには使わない。

## ツール

名前は `list_learning_records`、`save_learning_record`、`get_learning_record` で確定（2026-11-12）。旧草案の名前は採用しない。

| 名前 | 呼ぶもの | 入力 | 成功時の出力 |
|---|---|---|---|
| `list_learning_records` | `GET /learning-records` | `limit`（省略時20、1〜100）、`offset`（省略時0）。所有者もトークンも取らない | `records`（`id`・`theme`・`created_at`）と `has_more`。0件も成功。問題本文は返さない |
| `save_learning_record` | `POST /learning-records` | `request_id`・`theme`・`questions`（1〜10問、現行の `free_text`）。`Content-Type: application/json`。最大1 MiB | `id`・`request_id`・`theme`・`questions`・`created_at`。新規も同一内容の再送も200。再送は件数を増やさない |
| `get_learning_record` | 既存の `GET /learning-records/{id}` | `id`（記録のUUID）。所有者もトークンも取らない | `id`・`request_id`・`theme`・`questions`・`created_at`。問題・回答・フィードバックを含む。新しいサーバーAPIは作らない |

一覧の200は、未完了の再接続ならサーバーが確認状態にする。アダプターはそれを別の操作にしない。

## 失敗をAIツールへ返すもの

サーバーのHTTPステータスと `{"error":"コード"}` を、そのまま短く返す。トークン、SQL、受信本文は返さない。401と409は自動で再試行しない。トークンを新しく作る案内もしない。発行はWebの本人が行う。

| ツール | HTTP | コード | AIツールへ返す意味 |
|---|---|---|---|
| 3つ | 401 | `unauthorized` | トークンが無い、不正、無効、期限切れ、失効。どのツールも成功していない |
| 3つ | 500 | `internal_error` | サーバー内部の失敗。成功したとは返さない |
| 一覧 | 400 | `invalid_pagination` | `limit` か `offset` が契約外 |
| 保存 | 400 | `invalid_json` | 本文の構文、型、未知項目 |
| 保存 | 400 | `validation_failed` | 入力規則違反。`details` も返す |
| 保存 | 409 | `request_conflict` | 同じ要求IDで内容が違う。元の記録は変えない |
| 保存 | 413 | `request_too_large` | 本文が1 MiBを超えた。保存しない |
| 保存 | 415 | `unsupported_media_type` | JSONではない |
| 詳細取得 | 400 | `invalid_record_id` | `id` がUUIDではない |
| 詳細取得 | 404 | `record_not_found` | 本人の記録が無い。他人の記録も同じ応答 |

一覧と詳細取得に409は無い。再接続開始の `token_unavailable` は、この3ツールの応答ではない。

## 通信方式

stdio（確定、2026-11-12）。AIツールが使うときだけアダプターを起動し、標準入出力でやり取りする。待ち受けるポートは開けない。

ローカルHTTPとリモートMCPは今回採用しない。リモートMCPは計画v2（[docs/implementation-plan-v2.md](../implementation-plan-v2.md)）の最初の範囲外。

## 未決

なし。
