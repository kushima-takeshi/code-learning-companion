# 現在の作業の記録（2026-11）

2026-11-04の整理で [現在の作業](../current-work.md) から移した、2026-11の完了した日付付き記録。履歴であり、現在地・未決の正本ではない。
本文は移動時のままのため、文中の相対リンクは元の `docs/current-work.md` の配置を前提としている。

## 前回PR #26の確認結果（2026-11-01）

- `Service.List(ctx, userID, limit, offset)`、`RecordSummary`、`ListPage`を追加。本人限定・保存日時とIDの降順・空一覧・上限と末尾・DB障害を実DBで確認した。
- `connectiontoken.Service.RequireToken`と専用コンテキストを追加。`GET /learning-records`へ接続し、Webの本人認証とは分離した。初期20件・最大100件、開始位置`offset`、応答の`has_more`で続きの有無を示す。カーソル方式や特定のAIツールに固定しない。
- 実DBのHTTPテストで、本人分離、0件、20/100件、ページ末尾、他人のCookieを同時送信した場合、原本・本文・所有者情報の非公開、不正クエリ、無効・期限切れ・失効済み、認証を先に行うこと、照合／一覧DB障害、管理APIの拒否を確認した。読むだけで期限を延長しない。
- Docker内で`LEARNINGRECORD_TEST_DATABASE_URL`と`CONNECTIONTOKEN_TEST_DATABASE_URL`を指定し、`go test ./... -count=1`が成功。`go vet ./...`・gofmt・`git diff --check`・更新文書のローカルリンク確認も成功。他パッケージの外部DB・MinIOを使うオプトイン統合テストは未設定・未実行。
- 通常開発DBはversion=4、dirty=falseのまま。テスト用スキーマ残存0。マイグレーション追加・通常DBへの5適用・実行中APIの再起動・ブラウザ／MCP操作は行っていない。
- 実装コミットは`b1cf503`（内部一覧取得）、`249e4c5`（Bearer認証とHTTP入口）。仕様・検証記録は`76b1413`として分離した。3コミットをプッシュしてPR #26を作成済み。PR作成記録はさらに別の文書コミットにする。ロードマップ・処理フロー・クラス図を現在のコードと照合し、画像として作成済み。ローカル説明画像の`output/`はコミット対象外。

## レビュー中の修正：offsetの32ビット制約を撤回（2026-11-01）

- ユーザーの指摘を受け、業務上の根拠がなかった`MaxListOffset`を削除した。内部の一覧取得は`offset >= 0`を検証し、HTTP入力は`strconv.Atoi`でGoの`int`へ変換する。1〜100件の`limit`、負数・不正形式・重複・未知項目の拒否は維持する。
- 32ビットを超える開始位置と実行環境の`int`最大値を受け付ける回帰テストを先に追加し、旧実装でFAILすることを確認した。修正後、64ビット環境で`offset=2147483648`と`int`最大値のHTTP取得が200の空一覧になること、`int`を超える値は400になることを実DBで確認した。
- 学習記録パッケージ、および学習記録・連携トークンの実DB接続先を指定した`go test ./... -count=1`が成功。`go vet ./...`も成功。巨大な開始位置の負荷対策・保存件数の制限とは別の変更であり、これらの性能保証や業務上限を追加したものではない。
- 元のコミットの解説中に見つかった修正として、履歴を書き換えず追加コミットでPR #26へ反映する。

## 保存・詳細取得APIの確認結果（2026-11-01）

- 保存は`POST /learning-records`。既存の`SaveInput`・`DecodeSaveInput`・`Save()`へ接続し、読み込み段階で1 MiBに制限した。1〜10問を受け付け、未知項目・不正JSON・検証失敗では保存しない。
- 保存成功と同じ内容の再送は200で、所有者IDを除いた`id`・`request_id`・`theme`・`questions`・`created_at`を返す。同じ要求IDで異なる内容は409。順次再送・6件の同時再送で重複せず、競合で元記録が変わらないことを実DBで確認した。
- 詳細取得は`GET /learning-records/{id}`。保存応答と同じ形式の本文を返す。他人と存在しないIDは同じ404、不正UUIDは400、認証とDB障害は401と500に分ける。
- 両入口は既存の連携トークン認証で保護。認証を本文読み込み・ID検証より先に行い、Web Cookieだけでは拒否する。異なる所有者のCookieを同時送信してもBearerの本人で保存・取得する。所有者情報を応答せず、有効期限は延長しない。
- TDDで、保存テストは未実装時の405、詳細テストは未実装時の404でFAILし、各実装後にPASSした。1/10問、0/11問拒否、本文容量、未知項目、エラー項目位置、順次・同時再送、競合、本人分離、認証拒否、照合・保存・取得DB障害を実DBで検証した。
- Docker内で学習記録と連携トークンの実DB接続先を指定した`go test ./... -count=1`、`go vet ./...`が成功。他パッケージの外部DB・MinIOを使うオプトイン統合テストは未設定・未実行。
- 今回の保存・詳細HTTPテストを対象に`go test -race ./internal/learningrecord -run 'TestLearning(Save|Get)' -count=1`が実DBで成功。gofmtの差分なし、`git diff --check`、更新文書のローカルリンク確認も成功。通常開発DBはversion=4・dirty=false、race検査後もテスト用スキーマ残存0。
- 実装コミットは`0082f1f`（保存API）、`0b4831e`（詳細取得API）。仕様・検証記録は`fcca33e`に分けた。通常開発DBへのマイグレーション適用、実行中APIの再起動、ブラウザ・MCPでの操作は行っていない。

前PRから引き継いだレビュー状況（当時のメモを保全。マージ済みという事実とは区別する）:

[PR #26](https://github.com/kushima-takeshi/code-learning-companion/pull/26)のレビューは、内部一覧取得と認証・HTTPハンドラまでユーザー確認済み。`rows.Scan`の書き込み、`limit+1`での続きの判定、クエリ解析・重複／空値の拒否・`Atoi`による整数変換を説明し、ユーザーからハンドラ確認OKの返答を得た。次は`76b1413`の仕様・検証記録を確認する。文書の説明を提示しただけでレビュー完了とは扱わない。

## 2026-11-01: GitHub ActionsのCI

- ユーザー依頼により、PRとmainへのpushで動く最小のCIを`.github/workflows/ci.yml`に追加した。機能開発の現在地（接続UXの後続）は変えていない。アプリのコードは変更していない。
- バックエンドはGo 1.26.10で`go vet ./...`、`gofmt -l`の差分検査、`go test ./... -count=1`を実行する。Postgres 16をサービスコンテナで起動し、`LEARNINGRECORD_TEST_DATABASE_URL`と`CONNECTIONTOKEN_TEST_DATABASE_URL`をdocker-composeと同じ`clc`ユーザー・`clc`データベースへ向ける。学習記録と連携トークンのテストは、専用スキーマを作り`migrations`のSQLを自分で適用する。
- ローカルのPostgreSQL 16で同じ環境変数を指定し、`go test ./... -count=1`、`go vet ./...`、gofmt差分なしを確認した。スキップは既存条件どおり`TestImportIntegration`（`PRIMPORT_INTEGRATION_TEST=1`とMinIO）と`TestMinIOPutDelete`（`MINIO_INTEGRATION_TEST=1`）のみ。
- フロントエンドはNode 22で`npm ci`、`npm run lint`、`npm run build`が成功した。テストスクリプトはpackage.jsonにない。lintは`src/App.tsx`の警告1件で終了コード0。
- ブランチ保護でCI成功を必須にする設定は、リポジトリ設定であり、この作業では変更していない。
- GitHub Actionsの[実行 41298028013](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41298028013)で、ジョブ`backend`と`frontend`が成功した。`go test`はconnectiontoken、learningrecord、github、primport、storageがok。PR取り込みとMinIOの統合テストはオプトイン変数を付けていないので、既存のSkipに従う。フロントエンドはlintとbuildが成功し、ビルドは92msで完了した。PRは[#27](https://github.com/kushima-takeshi/code-learning-companion/pull/27)。


## PR #28作成記録（2026-11-01）

- 保存・詳細取得APIの3コミットをプッシュし、[PR #28](https://github.com/kushima-takeshi/code-learning-companion/pull/28)をmain向けに作成した。open・未マージ、意図した13ファイル、プッシュ済みheadを確認。GitHub連携ツールは作成権限不足のため、既存のGitHub CLI認証で作成し、このチャットへ添付した。
- 全体ロードマップ、処理フロー、関連型のクラス図を`output/save-detail-*.png`に作成して表示とコードを照合した。クラス図ではHandlerのSave/Get、既存Service・DB、入力・内部Record・公開応答・問題と回答・フィードバック・検証エラーの関係、通常関数とメソッドの区別を確認した。画像はローカル説明用でPRに含めない。
- 実装・仕様コミット`fcca33e`をheadとするCIの[実行 41316595567](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41316595567)でbackend・frontendとも成功。PR作成記録の文書コミット`714e447`反映後も、[実行 41316699066](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41316699066)でbackend・frontend両方の成功を確認済み。
- PR作成記録を別の文書コミットにし、マージは行わない。未コミットだった前PRのレビュー引き継ぎ内容を今回の進捗文書に保全済みで、作業前のstashも保持している。


## PR #28の確認と引き継ぎ（2026-11-02）

- 保存APIの`0082f1f`は、保存ルート・Handler.Save・公開応答・テストを解説し、ユーザーから「OK、次」の返答を得た。
- 詳細取得APIの`0b4831e`は、ルート・UUID検証・本人限定取得・404と500の区別・テストを解説し、ユーザーから「これもOK、次」の返答を得た。コード2コミットの解説・ユーザー確認は完了。
- ユーザーから「文章の解説はしなくていい」と指示があったため、文書コミット`fcca33e`とPR作成記録`714e447`の追加解説は不要。文書の個別レビュー完了とは区別する。
- 次回も参照できるよう、この文書の現在地・次の一手・確認状況を整理するよう依頼された。今回変更したのはこの文書のみで、アプリのコード変更・テスト再実行・コミット・プッシュ・マージは行っていない。
- 続いてユーザーから、この文書の更新をコミット・プッシュしてPR #28へ含める依頼を受けた。PRがopen・未マージで、作業ブランチがPRのheadと一致することを確認し、この文書だけを反映対象とする。`output/`の説明画像・生成スクリプト等は従来どおりローカル説明用でコミット対象外。次回は`git status`とPR #28の最新状態を確認して作業を始める。

## PR説明の型（2026-11-02）

- 作業開始時の origin/main は、PR #28 のマージコミット `f51f8b0`。機能の現在地は変えていない。再接続・画面・MCPは未実装のまま。
- `.github/pull_request_template.md` を追加した。PR説明は、何を変えたか、なぜ変えたか、追加・変更したテストケースの一覧、どう確認したか、レビューで特に見てほしい点、変更規模の順で短く書く。
- テストケースは名前と、そのケースが確認する内容を1行で書く。期待値を変えたときはその点も書く。対応する `docs/specs/` の条件があれば同じ行に示す。人がテストコードを読まなくても、足りないケースに気づけるようにする。
- `AGENTS.md` に、PRを作るときはこの型で説明を書く旨を1項目追加した。
- アプリの Go / React コードは変更していない。手元の `go test` と frontend の lint / build は再実行していない。確認は PR の CI `backend` と `frontend` で行う。
- [PR #30](https://github.com/kushima-takeshi/code-learning-companion/pull/30) を main 向けに作成した。head `3fbbaa7` の CI [実行 41423741906](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41423741906) で `backend` と `frontend` が成功した。
- 「レビューで特に見てほしい点」の案内を更新した。仕様に明示がない判断を最優先にし、認証・トークン、削除・上書き、DBトランザクション、同時実行・再送、呼び出し側への影響、他のやり方があり得た設計を、ファイルと関数つきで書く。該当がなければ「なし」。「何を変えたか」「なぜ変えたか」の繰り返しは書かず、人の判断が必要な箇所だけを書く。head `fadfa4b` の CI [実行 41427566982](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41427566982) で `backend` と `frontend` が成功した。
- 本体コードがないPRでは、ルール・設定・テンプレート・文書で人の判断が必要な箇所をファイルと節で書く。「なし」は、それらも無いときだけ書く。head `2312220` の CI [実行 41428403864](https://github.com/kushima-takeshi/code-learning-companion/actions/runs/41428403864) で `backend` と `frontend` が成功した。
