# cfs

ネットワーク的に直接つながらない場所(NAT内など)のストレージを簡単にマウントするやつ. (にする予定)

## Usage

### ボリュームの登録

```console
cfs publish localpath user/volume
```

### ボリュームをマウント

一旦Windowsのみ．mountpointは未使用のドライブレターを指定する必要あり.

```console
cfs mount user/volume mountpoint
```

### 設定(未実装)

以下の環境変数で設定.

```
set CFS_HUB_URL="http://example.com:8080"
set CFS_HUB_TOKEN="dummy"
```


### サーバの起動

```console
cfshub -p 8080
```
