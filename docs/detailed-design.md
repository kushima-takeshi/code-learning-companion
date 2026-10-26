# Learning Companion 設計概要（第二案・現行）

更新日: 2026-10-26
[要件](../requirements.md)を実現する責任分担。開発順は[第二案](implementation-plan-v2.md)、現在地は[現在の作業](current-work.md)。

## 構成

```text
ユーザー ↔ AIツール内の自作スキル
                 ↕ 本人認証付き接続（MCPを提案）
              Goサービス ↔ PostgreSQL / Object Storage
                 ↕
           React管理・閲覧画面
                 ↓
             復習・通知処理
```

スキルがAIへ学習手順を指示する。Goは教材・学習結果を扱い、AI APIを呼ばない。
接続の実装方式は詳細設計で決める。Web Cookie、アプリ接続トークン、AI製品の認証は別物として扱う。

## 論理データ

| データ | 役割 |
|---|---|
| PRSnapshot / ConversationUpload | 保存済み教材と会話。既存実装を利用 |
| LearningCandidate / LearningSet | 候補と選択した学習対象 |
| LearningSession / Question | 初回・復習の回と、その回の問題 |
| AnswerAttempt / Evaluation | 提出内容と判定・解説。ヒント・自己評価・テスト結果を区別 |
| LearningNote | ユーザーが保存した要点・未解決点 |
| ReviewSchedule / Notification | 次の復習と通知の管理 |

これは論理上の役割。テーブル分割、型、API、状態enumは未確定。復習問題を初回へ上書きしない関係を詳細設計で定める。

## 共通ルール

- 認証済みユーザーから所有者を決め、関連データも同じ所有者であることを検証する。
- 教材の参照先と生成対象が一致することを検証する。内部Storageキーを外部指定させない。
- 根拠は場所で保存し、表示時に教材から読み出す。問題用コードと引用元は区別する。
- AIの出力は検証してから保存する。回答は判定より先に保存する。
- 保存要求は冪等に扱い、同一IDで異なる内容が来た場合は競合として扱う。
- AIの評価は能力認定ではない。処理失敗と理解度の判定保留を分ける。
- サーバーが直接観測できないAI側の失敗を、確定したサーバー状態として推測しない。未提出・待機・再送の扱いを契約で決める。

## 機能仕様

| 仕様 | 対象 |
|---|---|
| [01](specs/01-authentication.md) | 既存Web認証。スキル接続認証は仕様03で追加設計 |
| [02](specs/02-public-pr-import.md) | 既存公開PR取り込み。本文読み出し・会話後付けは追加設計 |
| [03](specs/03-learning-questions.md) | スキルの学習手順と保存・再開 |
| [04](specs/04-review.md) | 復習・通知 |
| [05](specs/05-deep-dive.md) | AIツール内の深掘りと要点保存 |
| [06](specs/06-data-lifecycle.md) | 保持・削除・利用上限・公開条件 |

旧フェーズ3のHTTP生成API・BYOKの実装指示は利用しない。
既存取り込みの未決課題は[現在の作業](current-work.md)から参照する。
