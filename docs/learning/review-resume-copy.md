# 再開用の依頼文をコピーする

## 何のためか

「復習 2回目」などの欄で依頼文をコピーし、このアプリにMCPで接続したAIのチャットに貼り付けて、保存済みの記録から復習を進める。

コピーする文は次の1文。 `<id>` には詳細画面のパスをデコードした記録IDが入り、前後の空白や改行は付かない。

```text
get_learning_record で記録 <id> を取得し、その復習を進めてください。
```

## 状態の流れ

```mermaid
stateDiagram-v2
    [*] --> 表示待ち
    表示待ち --> コピー可能: 詳細を取得し期限一覧に記録IDがある
    表示待ち --> ボタンなし: 期限一覧に一致する行を取得できない
    コピー可能 --> コピー中: ボタンを押す
    コピー中 --> 成功: コピーしました
    コピー中 --> 失敗: コピーできませんでした
    成功 --> コピー中: もう一度押す
    失敗 --> コピー中: もう一度押す
```

## 呼び出しの流れ

1. `learning/LearningRecordDetailSection.tsx:loadDueReview` → `api/learningRecords.ts:listDueReviews`。パスの記録IDを `decodeURIComponent` し、一致した行を復習欄に使う。取得の詳細は [復習結果の画面](review-result-screen.md) を参照する。
2. `learning/LearningRecordDetailSection.tsx:LearningRecordDetailSection` のコピーボタン → `learning/LearningRecordDetailSection.tsx:handleCopyResumeRequest` → `navigator.clipboard.writeText`。一致した行の `id` を固定文に入れてコピーする。APIが無い場合や書き込みを拒否された場合は、同じエラーを表示する。
3. ユーザーが別チャットに貼り付けたあと、接続済みのAIが `id` を渡して [MCPの詳細取得ツール](mcp-get.md) を呼ぶ。コピー処理自体は通信やSQLを実行しない。

## 関数ごとの表

| 関数 | 目的 | 入力 | 出力 | 更新するもの |
|---|---|---|---|---|
| `loadDueReview` | 期限一覧から表示対象を見つける | パスのID、中断シグナル | なし | 一致した期限一覧の行、取得エラーまたはログイン状態 |
| `handleCopyResumeRequest` | 固定の依頼文をコピーする | 期限一覧で一致した記録ID | なし | クリップボード、ボタンの有効状態、成否表示 |

## なぜそうしたか

- コピーする内容を記録IDと指示に絞り、問題・回答・フィードバックは接続済みのAIがMCPで取得する。復習の段階は画面に表示する。
- 書き込みが終わるまでボタンを一時的に無効にする。成功時は `role="status"`、失敗時は `role="alert"` で伝え、どちらの結果でも押し直せる。

## 未決

期限が来た記録だけに表示する範囲は、詳細APIに段階を加える [Issue #88](https://github.com/kushima-takeshi/code-learning-companion/issues/88) までの暫定。
