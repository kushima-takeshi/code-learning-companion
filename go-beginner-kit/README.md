# Go初心者向け教材一式

Javaの経験を足場に、Goの文法から小さな学習記録APIまで学ぶ教材です。

**最初に開くファイル：`GO_BEGINNER_GUIDE.md`**

## 準備

公式サイトから現行の安定版Goをインストールしてください。コードが必要とする最低バージョンは1.22です。外部ライブラリは使っていません。

## 最初の実行

ターミナルで、このREADMEとgo.modがあるフォルダへ移動して実行します。

```sh
go version
go run ./cmd/01-hello
```

## 章ごとの実行

```sh
go run ./cmd/02-basics
go run ./cmd/03-functions
go run ./cmd/04-collections
go run ./cmd/05-structs
go run ./cmd/06-interfaces
go run ./cmd/07-json-defer
go test ./internal/study -v
go run ./cmd/api
```

APIはCtrl+Cで終了します。第10章は別途`go run ./cmd/10-concurrency`で動かします。

## 練習

`EXERCISES.md`の5つの実装課題と10個の理解確認に取り組んでください。解答説明は`ANSWERS.md`、動く模範解答は`answers`です。

## テストと整形

```sh
go fmt ./...
go test ./...
go vet ./...
go build ./...
go test -race ./...
```

最後のrace detectorは、対応するOS・アーキテクチャやCコンパイラ等の実行条件があります。

## APIの仕様

- `POST /records`：`{"title":"Go入門","minutes":25}`を登録。
- `GET /records`：一覧取得。
- `GET /records/1`：詳細取得。
- 保存先はメモリ。再起動で消えます。
- localhostでの学習用。更新・削除・認証・DB連携は含みません。

教材のコードは自由に変更して学習に利用してください。公式の仕様確認先は教材末尾にまとめています。
