# cfs

ネットワーク的に直接つながらない場所(NAT内など)のストレージを簡単にマウントするやつになる予定．

cfs: C***** File System

## TODO

- LinuxのFUSE対応
- WebUI
- ユーザ認証
- STUN+UDP対応
- 色々

## Usage

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
