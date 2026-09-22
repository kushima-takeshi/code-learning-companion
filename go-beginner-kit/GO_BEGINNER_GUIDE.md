# Java経験者のための、はじめてのGo

**文法を読む → 小さく動かす → テストする → 学習記録APIを作る**

作成日：2026年9月22日 ／ 対象：Go未経験・Javaの基本文法とAPI開発経験がある人

この教材はGoの最初の一歩から説明します。変数・関数・HTTPという考え方をJavaの経験に結び付けながら、Goでの書き方と設計の違いを学びます。プログラミング自体が初めての人は、各章を複数回に分けて進めてください。

題材は「学習記録」です。タイトルと学習時間を扱い、最後に登録・一覧・詳細取得の3つのAPIを動かします。DB・フレームワーク・外部ライブラリは使いません。

## この教材のゴールと進め方

読み終えたときに、次のことを自分の言葉で説明できれば合格です。

- `:=`、`[]Record`、`*Record`、`(Record, error)`を読める。
- 値のコピーと、ポインタ経由の更新を使い分けられる。
- interfaceが必要な場面と、具体型のままでよい場面を説明できる。
- 入力の不備を`error`で返し、HTTPのエラー応答につなげられる。
- 小さな関数とHTTPハンドラをテストできる。

時間は**合計15〜20時間程度**を目安にしてください。理解度に応じて調整するための配分であり、習得を保証する時間ではありません。1日45〜60分なら3〜4週間ほどです。

| 章 | 内容 | 目安 | 到達点 |
|---|---|---:|---|
| 1 | 環境準備・最初の実行 | 1時間 | `go run`を使える |
| 2 | 変数・型・分岐・ループ | 1時間 | 短い処理を書ける |
| 3 | 関数・複数戻り値・error | 1.5時間 | 正常と異常を返せる |
| 4 | スライス・map | 1.5時間 | 複数のデータを扱える |
| 5 | 構造体・ポインタ・メソッド | 2時間 | 更新対象を説明できる |
| 6 | interface・依存の渡し方 | 1.5時間 | 実装を差し替えられる |
| 7 | パッケージ・JSON・defer | 1.5時間 | APIの部品を読める |
| 8 | テーブル駆動テスト | 1.5時間 | 境界値を確認できる |
| 9 | 学習記録API | 3〜4時間 | 3つのAPIを動かせる |
| 10 | goroutine・contextの入口（発展） | 1時間 | 並行処理の注意点を説明できる |
| 復習 | 練習問題・作り直し | 1〜2時間 | 見ずに一部を書ける |

**1回の学習は「読む10分 → 実行10分 → 変更20分 → 説明5分」がおすすめです。** コードを眺めるだけで終わらず、実行前に結果を予想してください。翌日は前日のコードを見ずに、関数を1つだけ書き直します。

## ファイルの読み方

ZIPを展開した`go-beginner-kit`フォルダに、教材とすべてのコードがあります。

| ファイル・フォルダ | 用途 |
|---|---|
| `GO_BEGINNER_GUIDE.md` | この教材 |
| `README.md` | 実行の最短手順 |
| `EXERCISES.md` | プログラミング問題5問・理解確認10問 |
| `ANSWERS.md` | 解答の考え方・理解確認の解答 |
| `cmd/01-hello`〜`cmd/07-json-defer` | 第1〜7章の実行用コード |
| `internal/study` | 第8〜9章の入力チェック・保存処理・テスト |
| `cmd/api` | 第9章のHTTPサーバー・テスト |
| `cmd/10-concurrency` | 第10章の実行用コード |
| `answers` | プログラミング問題の解答コード・テスト |

本文の短いコードは、説明用の抜粋である場合があります。**そのまま実行できる完全なコードは各フォルダの`.go`ファイルです。** 特に指定のないコマンドは、`go.mod`のある`go-beginner-kit`フォルダで実行します。

---

## 第1章：まずGoを動かす

### 1-1. インストール

[Go公式のダウンロードページ](https://go.dev/dl/)から、お使いのOSに合う**安定版**をインストールします。`beta`や`rc`は選びません。[公式インストール手順](https://go.dev/doc/install)も参照してください。

Macでは「このMacについて」でチップを確認します。Apple MシリーズはARM64、IntelはAMD64用のmacOSインストーラーを選びます。WindowsではCPUに合うインストーラーを選びます。本教材のコマンド例はmacOS/Linuxのターミナルを基準にしています。

インストール後、新しいターミナルで確認します。

```sh
go version
```

`go version go1.xx.x ...`のように表示されれば準備完了です。

本教材のコードはGo 1.22以降の構文・APIを前提にしています。ただし、1.22を新たにインストールするという意味ではありません。インストールするのは公式サイトで提供される安定版にしてください。`go.mod`の`go 1.22.0`は最低限必要なGoバージョンを表します。

### 1-2. 最初のプログラム

ZIPを展開して、ターミナルでそのフォルダへ移動します。Finderからフォルダをターミナルへドラッグすると、パスを入力できます。

```sh
cd /自分が展開した場所/go-beginner-kit
go run ./cmd/01-hello
```

期待結果：

```text
Hello, Go!
```

実際のコードはこれだけです。

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Go!")
}
```

| 記述 | 意味 | Javaとの接点 |
|---|---|---|
| `package main` | 実行プログラムのパッケージ | 起動用クラスを用意する感覚に近い |
| `import "fmt"` | 表示などの機能を使う | `import`に相当するが、パッケージを取り込む |
| `func main()` | 実行開始位置 | `public static void main`の役割 |
| `fmt.Println` | 改行付きで表示 | `System.out.println`に近い |

Goでは、関数を必ずクラスに入れる必要はありません。文末のセミコロンも通常は書きません。

### 1-3. moduleとpackage

**packageはコードをまとめる単位、moduleは依存関係やバージョンを管理する単位**です。1つのmoduleに複数のpackageを置けます。

この教材は`example.com/go-beginner-kit`というmoduleです。これは学習用の識別名で、このドメインにアクセスしたり、公開したりする必要はありません。

教材には`go.mod`が入っているため、`go mod init`をやり直す必要はありません。自分で空のプロジェクトを作るときだけ、別のフォルダで次のように始めます。

```sh
mkdir hello-go
cd hello-go
go mod init example.com/hello-go
```

`go mod init`は`main.go`を作りません。自分でファイルを作ります。

### 1-4. 最初に覚えるコマンド

| コマンド | 役割 |
|---|---|
| `go run ./cmd/01-hello` | 指定したプログラムをビルドして実行 |
| `go build ./...` | module内の各packageをビルドできるか確認 |
| `go fmt ./...` | Goの標準的な書式に整形 |
| `go test ./...` | 全packageのテストを実行 |
| `go vet ./...` | 不審なコードを静的に確認 |

`./...`は現在のフォルダ以下のpackageを対象にする指定です。本教材のルートには起動用のGoファイルがないので、`go run .`ではなく、`go run ./cmd/01-hello`のように指定します。

**やってみる：** 表示する文字列を「今日からGoを学ぶ」に変えて実行します。第1章の到達点は、エディタで保存した変更が実行結果に反映されることです。

---

## 第2章：変数・型・条件分岐・ループ

実行：`go run ./cmd/02-basics`

### 2-1. 型は名前の後ろに書く

```go
var minutes int = 25
title := "Go入門"
const target = 60
```

Javaの`int minutes = 25;`に対し、Goは`var minutes int = 25`です。`:=`は型推論を使った**宣言と初期化**で、関数内で使います。既にある変数の値を変えるときは`=`です。

```go
minutes := 25
minutes = 30
```

同じスコープで`minutes := 30`をもう一度書くと、新しい変数がないためエラーになります。別の内側のスコープで同名を宣言すると、外側の変数を隠す「シャドーイング」になります。意図しない`:=`に注意してください。

### 2-2. ゼロ値

初期値を省略しても、変数には型に応じた値が入ります。

| 型 | ゼロ値 | 例 |
|---|---|---|
| `int`・`float64`など | `0` | 学習時間 |
| `bool` | `false` | 完了したか |
| `string` | `""` | タイトル |
| ポインタ・スライス・mapなど | `nil` | 対象やデータ構造が未設定 |

`nil`はすべての型に使える値ではありません。`int`や`string`に`nil`を代入することはできません。

### 2-3. ifとfor

```go
if minutes >= 60 {
    fmt.Println("目標達成")
} else {
    fmt.Println("学習中")
}

total := 0
for day := 1; day <= 3; day++ {
    total += 20
}
```

条件を丸括弧で囲む必要はありません。`{`は`if`や`for`と同じ行に書きます。繰り返しには`for`を使い、`for 条件 { ... }`とすればJavaの`while`に近い書き方になります。

`switch`も使えます。通常は一致したcaseだけを実行して終了するので、Javaでよく書く`break`は不要です。

```go
switch minutes {
case 0:
    fmt.Println("未学習")
case 25, 50:
    fmt.Println("区切りのよい時間")
default:
    fmt.Println("学習済み")
}
```

### 2-4. 型変換と表示

整数同士の割り算は整数になります。`5 / 2`は`2`です。小数を得たい場合は、割る前に型を変換します。

```go
average := float64(total) / 3
fmt.Printf("平均: %.1f分\n", average)
```

`%s`は文字列、`%d`は整数、`%v`は一般的な形式、`%T`は型の表示に使えます。Goでは`"時間: " + minutes`のような文字列と整数の連結はできません。`fmt.Sprintf("時間: %d", minutes)`などを使います。

サンプルの期待結果：

```text
Go入門: 25分
学習中
合計: 60分
平均: 20.0分
```

**練習：** `EXERCISES.md`のE1。境界値は`0`、`1`、`59`、`60`です。

---

## 第3章：関数・複数戻り値・error

実行：`go run ./cmd/03-functions`

### 3-1. 戻り値は引数の後ろ

```go
func add(a int, b int) int {
    return a + b
}
```

同じ型の引数は`func add(a, b int) int`とも書けます。戻り値がない場合は型の記述を省きます。

### 3-2. 失敗を戻り値で渡す

Javaでは例外で扱う場面でも、Goでは結果と`error`を返す形をよく使います。

```go
func parseMinutes(text string) (int, error) {
    minutes, err := strconv.Atoi(text)
    if err != nil {
        return 0, fmt.Errorf("minutes must be an integer: %w", err)
    }
    if minutes <= 0 {
        return 0, fmt.Errorf("minutes must be positive")
    }
    return minutes, nil
}
```

この関数の約束は「成功時は分数と`nil`、失敗時は`0`とエラー」です。失敗時の`0`を有効な学習時間として使ってはいけません。呼び出す側はまず`err`を確認します。

```go
minutes, err := parseMinutes("25")
if err != nil {
    fmt.Println(err)
    return
}
fmt.Println(minutes)
```

`err != nil`は「エラーがあるか」です。`error`は`Error() string`というメソッドを持つinterfaceです。最初は「失敗理由を持つ値」と捉えれば十分です。

`fmt.Errorf`の`%w`は元のエラーを包み、後から`errors.Is`や`errors.As`で調べられるようにします。`errors.Is(err, 対象のエラー)`は、包まれた原因もたどって確認します。文字列の一致でエラーの種類を判定するのは避けましょう。

### 3-3. 早期returnで正常処理を読みやすくする

「入力が不正なら戻る」を先に並べると、その後の処理は正常な入力を前提にできます。すべてを大きな`if ... else`で囲む必要はありません。

ユーザーの入力ミスのような、普通に起こり得る失敗には`error`を使います。`panic`をJavaの例外処理の代わりとして日常的に使うのは、この教材では扱いません。

期待結果：

```text
入力 "25": 25分
入力 "0": エラー
入力 "abc": エラー
```

**練習：** E2の平均関数を書きます。空の入力に対して、割り算する前にエラーを返してください。

---

## 第4章：スライスとmap

実行：`go run ./cmd/04-collections`

### 4-1. スライスは複数の値を扱う入口

```go
minutes := []int{20, 30}
minutes = append(minutes, 10)
for index, value := range minutes {
    fmt.Println(index, value)
}
```

`[]int`はintのスライス、`[3]int`は長さ3の配列です。配列は長さも型の一部です。まずは、要素を追加しやすいスライスを使いましょう。

Javaの`List<Integer>`と用途は近いですが、スライスは内部の配列への参照・長さ・容量を持ちます。同じ配列を複数のスライスが参照できる点が重要です。

`append`は結果のスライスを返すので、`minutes = append(...)`と受け取ります。追加で内部配列が変わる場合もあるため、戻り値を無視してはいけません。

`range`の添字を使わないときは、`for _, value := range minutes`と書きます。`_`は受け取った値を使わないことを表します。

### 4-2. コピーしたつもりが共有される

```go
a := []int{10, 20}
b := a
b[0] = 99
fmt.Println(a) // [99 20]
```

コピーされたのはスライスの情報で、要素を保存する配列そのものではありません。独立した要素の並びが必要ならコピーします。

```go
c := make([]int, len(a))
copy(c, a)
```

`make`はスライス・map・channelなどを使うための初期化に使います。スライスの`len`は現在の要素数、`cap`はそのスライスから使える内部配列の容量です。

`var values []int`で作ったnilスライスでも、`len(values)`は0、`append(values, 1)`は利用可能です。ただし、要素がない状態で`values[0]`にアクセスするとpanicになります。

要素がポインタやmapなら、スライスのコピーだけでは参照先まで複製されません。本教材の単純な整数や、文字列・整数のみを持つRecordでは、このコピーで変更の共有を避けられます。

### 4-3. mapはキーで検索する

```go
byTopic := map[string]int{"Go": 60, "SQL": 0}
value, ok := byTopic["SQL"]
```

`map[string]int`は「キーがstring、値がint」です。存在しないキーを読むと値の型のゼロ値が返ります。そのため、`0`だけでは「登録済みで0」と「未登録」を区別できません。2つ目の戻り値`ok`を使います。

```go
value, ok := byTopic["Java"] // valueは0、okはfalse
```

`delete(byTopic, "Go")`で削除できます。mapの走査順は保証されません。順番が必要ならキーなどをソートしてください。nilのmapは読み取りできますが、要素を書き込む前に`make(map[string]int)`などで初期化する必要があります。

期待結果：

```text
合計: 60
SQL: 0 true
Java: 0 false
共有: [99 20]
コピー: [99 20] [1 20]
```

**練習：** E3。mapにまだキーがなければ値は0なので、`result[key] += value`で初回から合計できます。

---

## 第5章：構造体・ポインタ・メソッド

実行：`go run ./cmd/05-structs`

### 5-1. 関連する値をstructでまとめる

```go
type Record struct {
    Title   string
    Minutes int
}

r := Record{Title: "Go入門", Minutes: 20}
```

Javaのデータクラスに近い用途です。ただしGoにはJavaと同じclass・継承の仕組みはありません。構造体でデータをまとめ、型にメソッドを定義し、必要なら構成やinterfaceで振る舞いを組み合わせます。

### 5-2. Goの引数は値渡し

```go
func changeCopy(r Record) {
    r.Minutes = 999
}
```

`changeCopy(r)`を呼ぶと、関数にはRecordのコピーが渡ります。そのコピーを書き換えても、呼び出し元の`r.Minutes`は変わりません。

Javaも引数は値渡しですが、オブジェクトの場合に渡す値は参照です。Goのstructをそのまま渡した場合はstruct自体がコピーされるので、ここを区別してください。

### 5-3. &と*を読む

| 書き方 | 意味 |
|---|---|
| `&r` | 変数rを指すポインタを得る |
| `*Record` | Recordを指すポインタ型 |
| `*p` | ポインタpが指している値を参照する |
| `p.Minutes` | pが指すstructのフィールドへアクセスする省略記法 |

```go
p := &r
p.Minutes = 40
```

これは元のrを書き換えます。ポインタを引数で渡す場合も「ポインタという値」のコピーが渡されますが、指している先が同じなので元の値を更新できます。Goで通常のポインタ演算を行うことはできません。

### 5-4. メソッドとレシーバ

```go
func (r Record) Summary() string {
    return fmt.Sprintf("%s: %d分", r.Title, r.Minutes)
}

func (r *Record) AddMinutes(minutes int) {
    r.Minutes += minutes
}
```

関数名の前の`(r Record)`や`(r *Record)`がレシーバです。どの型に対する操作かを示します。

`Summary`は値レシーバなのでRecordのコピーで動きます。`AddMinutes`はポインタレシーバなので元のRecordを更新します。アドレスを取れる変数rに対しては、`r.AddMinutes(10)`と書けます。必要なポインタ化をGoが補います。

最初の使い分けは「元を更新するならポインタ」です。ただし、読み取りメソッドでも、大きなstructのコピーを避けたり、型全体のレシーバをそろえたりするためにポインタを使うことがあります。`sync.Mutex`を含む値は、使用開始後にコピーしないようにします。

期待結果：

```text
Go入門: 20分
Go入門: 30分
Go入門: 40分
```

**練習：** E4。不正な分数なら元の値を維持してください。「検証してから代入する」順番が重要です。

---

## 第6章：interfaceと依存の渡し方

実行：`go run ./cmd/06-interfaces`

### 6-1. 必要な操作を定義する

「一覧を取得できればよい」という処理に、具体的な保存方法まで知る必要はありません。

```go
type RecordReader interface {
    List() []Record
}

func printTitles(reader RecordReader) {
    for _, record := range reader.List() {
        fmt.Println(record.Title)
    }
}
```

`printTitles`は`List() []Record`を呼べる相手を受け取ります。メモリ保存かDB保存かは、この関数の関心事ではありません。

### 6-2. implementsは書かない

```go
type MemoryReader struct {
    records []Record
}

func (m MemoryReader) List() []Record {
    return append([]Record(nil), m.records...)
}
```

この型は必要なメソッドを持っているため、RecordReaderとして使えます。`implements RecordReader`という宣言はありません。

メソッド名だけでなく、引数と戻り値の型も一致する必要があります。また、`List`を`*MemoryReader`のポインタレシーバに定義した場合、interfaceに渡せるのは基本的に`*MemoryReader`です。`MemoryReader`の値を渡しても、自動的にinterfaceを満たすわけではありません。

### 6-3. SpringのDIと結び付ける

`printTitles(reader)`のように依存する相手を外から渡せば、Goでも依存性注入ができます。DIという考え方に、コンテナやアノテーションが必須なわけではありません。

小さなプログラムでは`main`で必要な部品を作り、コンストラクタ相当の関数や通常の引数で渡せば十分です。Goの`NewStore()`のような`New...`関数は慣習的な命名で、特別な構文ではありません。

**何でもinterfaceにしなくて大丈夫です。** テストで差し替えたい、複数の実装がある、呼び出す側の依存を限定したい、といった理由があるときに小さく定義します。第9章では、保存処理は1種類なので`*study.Store`を直接渡しています。

期待結果：

```text
Go入門
SQL復習
```

**練習：** E5。実際のDBを用意せず、固定の一覧を返す型で合計処理をテストします。

---

## 第7章：パッケージ・JSON・defer

実行：`go run ./cmd/07-json-defer`

### 7-1. 公開範囲は先頭文字で決まる

Goでは、名前の先頭を大文字にすると、他のpackageから参照できる公開名になります。

| 名前 | 他packageからの参照 |
|---|---|
| `Record`・`Validate`・`Title` | 可能 |
| `record`・`validate`・`title` | 不可 |

小文字のフィールドも、同じpackageからは参照できます。Javaのクラス単位のprivateと同じではありません。

原則として、同じディレクトリの通常のGoファイルは同じpackageにします。ファイルが別でも同じpackageなら、importなしで互いの関数を使えます。テスト用packageなどの例外はありますが、最初はこの原則で十分です。

`internal/study`は、教材moduleの中から使うpackageです。`internal`というディレクトリ名には、親ディレクトリのツリー外からのimportを制限する仕組みがあります。まずは「外へ提供しない実装をまとめる場所」と理解してください。

### 7-2. JSONタグで外部向けの名前を決める

```go
type Record struct {
    Title   string `json:"title"`
    Minutes int    `json:"minutes"`
}
```

バッククォート内の文字列はstructタグです。標準の`encoding/json`がそれを読み取り、`Title`を`title`というJSONキーに変換します。

```go
data, err := json.Marshal(r)
```

`Marshal`はGoの値をJSONのバイト列`[]byte`へ変換します。表示するときは`string(data)`にします。

```go
var decoded Record
err := json.Unmarshal(data, &decoded)
```

`Unmarshal`はJSONをGoの値へ読み込みます。`decoded`に値を書き込んでほしいので、`&decoded`を渡しています。標準JSON変換で対象にするstructフィールドは、先頭を大文字にしてください。

`[]Record(nil)`はJSONで`null`、要素数0の非nilスライスは`[]`になります。APIの一覧応答では、この違いを意識しましょう。

### 7-3. deferは関数終了時の後始末

```go
defer fmt.Println("最後に実行")
defer fmt.Println("先に実行")
```

`defer`で登録した呼び出しは、周囲の関数が終了するときに実行します。複数ある場合は、後に登録したものから実行します。

典型例は、ファイルを開くのに成功した直後に`defer file.Close()`を書くことです。途中でreturnしても後始末できます。`defer`は「ブロック終了時」ではないので、大量のループ内で使うと、関数が終わるまで後始末がたまることがあります。

また、`defer`に渡す引数は登録した時点で評価されます。

```go
n := 1
defer fmt.Println(n)
n = 2
// 関数終了時に出るのは1
```

期待結果：

```text
{"title":"Go入門","minutes":25}
復元: Go入門 / 25分
先に実行
最後に実行
```

**確認：** `Unmarshal`に`&decoded`を渡す理由を、第5章の内容を使って説明してください。

---

## 第8章：テストで仕様を確かめる

実行：

```sh
go test ./internal/study -v
```

### 8-1. テストの形

テストは`study_test.go`のように`_test.go`で終わるファイルに書きます。`testing`をimportし、`func TestXxx(t *testing.T)`という関数を作ります。

```go
func TestValidate(t *testing.T) {
    err := Validate("Go", 25)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

これは説明用の最小例です。教材の実際の`TestValidate`は、次のテーブル駆動方式を使っています。既存ファイルへ同名関数を追加しないようにしてください。

### 8-2. テーブル駆動テスト

入力と期待結果を一覧にして、同じ確認処理を繰り返す書き方です。JUnitのパラメータ化テストと役割が近いです。

```go
tests := []struct {
    name    string
    title   string
    minutes int
    wantErr bool
}{
    {"normal", "Go", 25, false},
    {"empty title", "", 25, true},
    {"zero", "Go", 0, true},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        err := Validate(tt.title, tt.minutes)
        if (err != nil) != tt.wantErr {
            t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
        }
    })
}
```

`(err != nil)`は「実際に失敗したか」、`tt.wantErr`は「失敗するはずか」です。この2つが一致しなければテストを失敗させます。

### 8-3. テストするのは実装の行ではなく仕様

この教材の入力ルールは次の通りです。

- タイトルは前後の空白を取り除いて保存する。
- 空白除去後のタイトルは1〜100 Unicodeコードポイント。
- 分数は整数で1〜1440。

境界の内側・外側を対にしてテストします。`0 / 1`、`1440 / 1441`、タイトルの`100 / 101`です。

Goの`len(string)`はバイト数です。日本語の文字数制限をそのまま`len`で書くと、意図とずれます。この教材では`utf8.RuneCountInString`を使っています。ただし、結合文字や絵文字などでは、Unicodeコードポイント数と見た目の文字数は一致しません。仕様も「コードポイント数」として決めています。

`Validate`は渡されたタイトルの長さを確認します。保存処理`Store.Add`が先に空白除去をしてから呼ぶので、APIでは空白除去後の長さが判定対象になります。

### 8-4. 小さく失敗させて、直す

一度、`study.go`の`minutes < 1`を`minutes < 0`に変えてみてください。`zero`ケースが失敗するはずです。確認後は元に戻します。

テストの表示では、`PASS`や`ok`が成功です。`[no test files]`は、そのpackageにテストがないことを表し、失敗ではありません。

---

## 第9章：学習記録APIを作る

### 9-1. できあがりの仕様

| メソッド | パス | 内容 | 成功時 |
|---|---|---|---|
| POST | `/records` | タイトル・分数を登録 | 201と登録したRecord |
| GET | `/records` | 登録順に一覧取得 | 200と配列 |
| GET | `/records/{id}` | IDで1件取得 | 200とRecord |

保存先はメモリです。**再起動するとデータは消えます。** 更新・削除・DB・ログインは含みません。サーバーは`127.0.0.1`で待ち受け、自分のPCから学習用にアクセスします。

| ファイル | 責任 |
|---|---|
| `cmd/api/main.go` | 部品を作ってサーバーを起動する |
| `cmd/api/handlers.go` | HTTP・JSONの入出力、ステータスへの変換 |
| `internal/study/study.go` | 入力ルール、ID採番、保存・検索 |
| `cmd/api/handlers_test.go` | APIの正常系と異常系を確認 |

SpringのControllerに近い役割をHTTPハンドラが担当します。業務の入力ルールは`study.Validate`へ置き、HTTPを知らなくてもテストできるようにしています。小さな例なので、ServiceやRepositoryの層を機械的に増やしてはいません。

### 9-2. まず完成形を動かす

ターミナルAで起動します。

```sh
go run ./cmd/api
```

`listening on http://127.0.0.1:8080`と表示され、待機します。これが正常です。止めるときはCtrl+Cです。

ターミナルBで次のコマンドを実行します。Windows PowerShellでは必要に応じて`curl.exe`を使い、JSON文字列の引用方法をシェルに合わせて調整してください。

**登録前の一覧：**

```sh
curl -i http://127.0.0.1:8080/records
```

ステータスは200、本文は`[]`です。

**登録：**

```sh
curl -i -X POST http://127.0.0.1:8080/records \
  -H 'Content-Type: application/json' \
  -d '{"title":"Go入門","minutes":25}'
```

ステータスは201、`Location: /records/1`が付き、本文は次の通りです。最初の登録を想定しています。

```json
{"id":1,"title":"Go入門","minutes":25}
```

**詳細と一覧：**

```sh
curl -i http://127.0.0.1:8080/records/1
curl -i http://127.0.0.1:8080/records
```

一覧は`[{"id":1,"title":"Go入門","minutes":25}]`です。もう1件登録するとIDは2になります。

**入力エラー：**

```sh
curl -i -X POST http://127.0.0.1:8080/records \
  -H 'Content-Type: application/json' \
  -d '{"title":"Go入門","minutes":0}'
```

ステータスは400、本文は次の通りです。

```json
{"error":"minutes must be between 1 and 1440"}
```

### 9-3. ハンドラの入口を読む

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /records", func(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, store.List())
})
```

`mux`はHTTPメソッド・パスと処理を結び付けます。`w`は応答を書き出す相手、`r`は受け取ったリクエストです。無名関数は外側の`store`を使っています。このように周囲の変数を参照する関数をクロージャと呼びます。

`"GET /records"`や`"GET /records/{id}"`という登録方法はGo 1.22以降の標準ServeMuxで使えます。`r.PathValue("id")`でパスの値を取り出します。GET用のパターンはHEADにも一致します。

### 9-4. 登録処理を5つに分解する

1. Bodyのサイズを制限し、JSONを読む。
2. 余分なJSON値や未知のフィールドを拒否する。
3. `store.Add`で空白除去・入力チェック・保存を行う。
4. 失敗なら400を返して終了する。
5. 成功ならLocationと201、登録データを返す。

JSONの読み取りでは、次の設定を使っています。

```go
decoder := json.NewDecoder(r.Body)
decoder.DisallowUnknownFields()
var input createRequest
err := decoder.Decode(&input)
```

未知のフィールドを拒否すると、`minutes`のつもりで`minute`を送った誤りを発見できます。さらに2回目のDecodeで`io.EOF`を確認し、`{...} {...}`のようにJSON値が続いている入力を拒否します。

教材では不正なJSON、サイズ超過、入力ルール違反をいずれも400にまとめています。サイズ超過を413、Content-Typeの不一致を415に分ける処理は含みません。また、標準`encoding/json`は重複キーを一律に拒否しません。この教材も重複キーの独自検出はしていません。

### 9-5. なぜ保存処理にMutexが必要か

HTTPサーバーには複数のリクエストが同時に来ます。`nextID`やスライスを無保護で更新すると、データ競合が発生し得ます。

```go
s.mu.Lock()
defer s.mu.Unlock()
```

Mutexで共有データへの操作を1つずつ進めます。Lockは保護区間へ入る操作、Unlockはそこを出る操作です。`defer`によって途中でreturnしてもUnlockされます。第10章を終えるまでは、まずこの保護が必要だという点を押さえてください。

一覧を返すときはスライスをコピーします。内部のスライスをそのまま返すと、呼び出し元が要素を変更できてしまい、Mutexで保護する境界も崩れてしまうからです。

### 9-6. エラーとHTTPステータスを対応させる

| 状況 | ステータス | 例 |
|---|---:|---|
| 正常な取得 | 200 | 一覧・存在するID |
| 登録成功 | 201 | 有効なPOST |
| JSON不正・入力不正・ID形式不正 | 400 | 分数0、`/records/abc` |
| 有効な形式のIDだが未登録 | 404 | `/records/999` |
| 未定義のパス | 404 | `/unknown` |
| そのパスに対応しないメソッド | 405 | `DELETE /records` |

入力の形式が不正なのか、検索結果が存在しないのかを分けています。`study.ErrNotFound`を`errors.Is`で判定し、HTTP層で404に変換します。

本教材で明示的に返すエラーはJSONです。一方、ServeMuxが自動的に返す未定義パスの404やメソッド不一致の405は標準のテキスト応答です。API全体でエラー形式を統一するなら、追加の設計が必要になります。

### 9-7. サーバーを起動せずハンドラをテストする

```go
req := httptest.NewRequest("GET", "/records", nil)
w := httptest.NewRecorder()
handler.ServeHTTP(w, req)
```

`httptest`でリクエストと応答の記録先を用意します。実際に8080番ポートを開かずに、ステータスやJSON本文を確認できます。

```sh
go test ./cmd/api -v
go test ./...
```

確認するのは、成功ステータスだけではありません。登録値を取得できること、異常な入力で保存内容が増えないこと、存在しないIDが404になることも確認します。

### 9-8. 自分で書く順序

完成版を一度実行したら、別の練習用フォルダで次の順番に再現します。

1. `Record`と`Validate`を書き、境界値のテストを通す。
2. `Store.Add`・`List`・`Find`を書く。
3. GET一覧を実装し、空の`[]`を返す。
4. POSTを実装し、正常入力と不正入力を確認する。
5. GET詳細と404を実装する。
6. `httptest`で登録から取得までの一連の流れを確かめる。

一度にすべて写すより、1つの動作を説明できる状態で次に進んでください。完成コードを参照して構いませんが、必ず「この関数は何を受け取り、何を返すか」を声に出して確認します。

---

## 第10章：goroutine・channel・contextの入口

ここは発展編です。第9章までに自信がなければ、後回しにして構いません。

実行：`go run ./cmd/10-concurrency`

### 10-1. goroutineは並行して動く処理

```go
go someFunction()
```

`go`を付けると、新しいgoroutineで関数を実行します。goroutineとOSスレッドは1対1の対応ではなく、Goランタイムがスケジュールします。

起動した順に終了するとは限りません。また、main関数が終わると、他のgoroutineの完了を自動では待たずプログラムが終了します。「とりあえずsleepして待つ」ではなく、完了を受け取る仕組みを作ります。

### 10-2. channelで値を受け取る

```go
result := make(chan int)
go sum(ctx, []int{20, 30, 10}, result)
fmt.Println("合計:", <-result)
```

`chan int`はintを受け渡すchannelです。`result <- total`で送信、`<-result`で受信します。このバッファなしchannelでは、送信側と受信側がそろうまで待機します。サンプルは結果を受信するので、計算が終わる前にmainが終了するのを防げます。

関数の引数にある`chan<- int`は、関数側から送信だけできるchannelを表します。必ずchannelをcloseしないといけないわけではありません。このサンプルは1回だけ結果を受け取る約束なのでcloseしていません。

### 10-3. contextは処理をやめてほしいという合図

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

contextはキャンセルや期限を伝えます。`cancel()`を呼ぶだけで、任意の処理を強制終了できるわけではありません。処理側が`ctx.Done()`を監視したり、context対応のAPIを使ったりする必要があります。

```go
select {
case result <- total:
case <-ctx.Done():
    return
}
```

`select`は進められるchannel操作を選びます。受信側がいなくなった場合でも、キャンセルに反応して送信待ちをやめられます。

サンプルでは結果を受け取った後にキャンセルします。もし先にキャンセルするよう変更するなら、main側の受信も`select`でキャンセルに対応させてください。結果を送らず終了した処理を`<-result`だけで待つと、待ち続ける原因になります。

期待結果：

```text
合計: 60
```

わずか3件の合計に並行処理は必要ありません。ここでは仕組みの学習のために使っています。処理を速くする目的で安易にgoroutineを増やさないでください。

### 10-4. データ競合を検出する

```sh
go test -race ./...
```

race detectorは、実際に実行されたコードの共有メモリアクセスを調べます。通過しても、すべての実行パターンに競合がないと証明したわけではありません。対応環境やCコンパイラ等の条件で実行できない場合は、通常の`go test ./...`と区別して結果を記録してください。

---

## Javaから移るときの確認表

| Javaで慣れていること | Goで最初に意識すること |
|---|---|
| クラスにメソッドを置く | packageの関数・struct・メソッドを使い分ける |
| `new`やコンストラクタ | structリテラルや通常の`New...`関数で初期化する |
| `public`・`private` | 先頭大文字なら他packageへ公開、小文字ならpackage内 |
| `implements` | 必要なメソッドがあれば暗黙にinterfaceを満たす |
| `try/catch` | 期待される失敗は戻り値のerrorを確認する |
| `List<T>` | スライスの内部配列共有に気を付ける |
| オブジェクト参照を渡す | struct値を渡すとコピーされる |
| Springが依存を組み立てる | mainなどで組み立て、引数で渡せる |
| JUnit | testing、テーブル駆動テスト、httptest |
| フォーマット設定 | まずgo fmtの結果にそろえる |

## よくあるエラーの直し方

| 表示・現象 | 最初に確認すること |
|---|---|
| `go: command not found` | インストール後にターミナルを開き直したか、PATHが通っているか |
| `go.mod file not found` | 教材のgo.modがあるフォルダへ移動したか |
| `no Go files` | `go run .`ではなく`go run ./cmd/01-hello`等を指定したか |
| `declared and not used` | 関数内で宣言した変数を使っているか |
| `imported and not used` | 不要なimportを残していないか |
| `no new variables on left side of :=` | 再代入なのに`:=`を使っていないか |
| `cannot use ... as ...` | 型、ポインタの有無、interfaceのメソッド定義が一致するか |
| `index out of range` | 長さ0のスライスへアクセスしていないか |
| `assignment to entry in nil map` | 書き込み前にmapを初期化したか |
| `address already in use` | 前のAPIサーバーが起動したままになっていないか |
| `connection refused` | ターミナルAのサーバーが起動しているか |
| 作ったAPIのパスが一致しない | Go 1.22以降か、古いMux互換設定を使っていないか |

## 学習記録のテンプレート

毎回、次の4行だけ残してください。量よりも、自分が何を理解し、どこで迷ったかを残すことが目的です。

```text
今日できたこと：
自分の言葉で説明すると：
まだ説明できないこと：
次回、見ずに書くコード：
```

## 次に学ぶこと

第9章まで自力で再現できたら、DB保存・contextを使う外部呼び出し・認証認可・終了処理へ進めます。DBを追加するときは、再起動してもデータが残ること、トランザクション、DBエラーの扱いが新しい学習対象です。

初回学習では、ジェネリクスの高度な型制約、reflection、unsafe、大規模な設計パターンまで一度に扱う必要はありません。まず、小さなAPIでデータとエラーの流れを説明できることを優先しましょう。

## 公式資料と参照箇所

公式資料を確認し、教材の説明・練習課題・コードはこの学習目的に合わせて作成しています。細かな仕様を確かめるときの参照先です。確認日：2026年9月22日。

| 内容 | 公式資料 |
|---|---|
| インストール | [Download and install](https://go.dev/doc/install) |
| 最初の実行・module | [Get started with Go](https://go.dev/doc/tutorial/getting-started) |
| 基本文法の追加練習 | [A Tour of Go](https://go.dev/tour/) |
| 型・代入・公開名などの厳密な規則 | [Go言語仕様](https://go.dev/ref/spec) |
| スライス | [Tour: Slices](https://go.dev/tour/moretypes/7) |
| interfaceの暗黙的実装 | [Tour: Interfaces are implemented implicitly](https://go.dev/tour/methods/10) |
| エラーのラップと判定 | [errors](https://pkg.go.dev/errors) |
| テストの導入 | [Add a test](https://go.dev/doc/tutorial/add-a-test) |
| JSON変換 | [encoding/json](https://pkg.go.dev/encoding/json) |
| HTTPルーティング | [net/http](https://pkg.go.dev/net/http) |
| HTTPテスト | [net/http/httptest](https://pkg.go.dev/net/http/httptest) |
| Unicodeコードポイント | [unicode/utf8](https://pkg.go.dev/unicode/utf8) |
| キャンセル | [context](https://pkg.go.dev/context) |
| 競合検出 | [Data Race Detector](https://go.dev/doc/articles/race_detector) |
