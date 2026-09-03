# MVP 設計概要

この文書はMVP全体の地図である。各機能の詳細な振る舞い・受け入れ条件は `specs/` 配下に分ける。

## 技術構成

```text
React + TypeScript → Go API → PostgreSQL / Object Storage
                              ├─ GitHub API（公開PR取得）
                              └─ OpenAI API（BYOK）
```

- GitHub OAuthはユーザー識別だけに使い、非公開リポジトリ権限を要求しない
- OpenAI APIキーは利用中のセッションだけで扱い、DB・ログ・Object Storageに保存しない
- 公開PRのみを扱う

## 機能仕様書

| 実装順 | 仕様書 | 対象 |
|---:|---|---|
| 1 | [01-authentication.md](specs/01-authentication.md) | GitHub OAuthとユーザー分離 |
| 2 | [02-public-pr-import.md](specs/02-public-pr-import.md) | 公開PR URL・Cursorファイルの取り込み |
| 3 | [03-learning-questions.md](specs/03-learning-questions.md) | OpenAI BYOK、候補、問題、回答、フィードバック |
| 4 | [04-review.md](specs/04-review.md) | 復習予定・再出題 |
| 5 | [05-deep-dive.md](specs/05-deep-dive.md) | 自由対話と会話ログ |
| 6 | [06-data-lifecycle.md](specs/06-data-lifecycle.md) | 保持、削除、利用上限、公開ベータ |

## 共通ルール

- すべてのアプリ内データは `user_id` で所有者を限定する
- PRは初回取り込み時のスナップショットを教材の正本とする
- LLMは設計意図を差分だけから断定しない
- LLM出力は構造化データとして検証し、根拠参照を持たない問題・フィードバックは保存しない
- APIキーをエラーログ・監査ログ・レスポンスへ含めない
- 詳細仕様書1つ、またはその中の小さな受け入れ条件群を、1つの実装PRの範囲にする

## 開発時の確認方法

AIへ実装を頼む前に、そのPRで扱う仕様書の「目的」「対象外」「受け入れ条件」を渡す。実装後は、変更ファイル、処理の流れ、採用理由、失敗時の確認方法を自分の言葉で説明できるか確認する。
