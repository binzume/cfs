# cfs

ネットワーク的に直接つながらない場所(NAT内など)のストレージを簡単にマウントするやつ. (にする予定)

## Usage


公開

```console
cfs publish localpath user/volume
```

マウント．

```console
cfs mount user/volume mountpoint
```

設定

```console
cfs config api http://localhost:8080
cfs config token TOKENSTRING1234567890
```

サーバの起動

```console
cfs-server -p 8080
```
