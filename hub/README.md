# cfs

ネットワーク越しに簡単にストレージを公開したりマウントしたりできるやつになる予定．

通信はwebsocketなのでhttp or httpsが通る場所ならだいたい使えます．

cfs: C***** File System

```
        +-----------+
        |  cfs hub  |
        +-----------+
          ^       ^
  publish |       | mount
 +-----------+  +-----------+
 |    cfs    |  |    cfs    |
 +-----------+  +-----------+
```

## TODO

- Linux/DarwinのFUSE対応をまともに
- WebUI
- ユーザ認証
- STUN+UDP対応
- 色々

## Usage

適当なところでhubを起動しておいてそこにクライアントからつなぎます(hubはクライアントから接続できる場所に置く必要があります)

### ボリュームの登録

```console
cfs publish localpath user/volume
```

### ボリュームをマウント

Windows以外は実装途中なのでまともに動きません. mountpointは未使用のドライブレターを推奨.

```console
cfs mount user/volume mountpoint
```

### サーバの起動

```console
cfshub -p 8080
```

### 設定

環境変数で設定.

```console
set CFS_HUB_URL=http://hub.example.com:8080
set CFS_HUB_TOKEN=dummy
```
