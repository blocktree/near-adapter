# near-adapter

本项目适配了openwallet.AssetsAdapter接口，给应用提供了底层的区块链协议支持。

## 如何测试

openwtester包下的测试用例已经集成了openwallet钱包体系，创建conf文件，新建Ⓝ.ini文件，编辑如下内容：

```ini
# node api url
nodeAPI = "https://rpc.mainnet.near.org/"

# https://docs.near.org/docs/concepts/gas
sendFoundsTokenBurnt = 42455506250000000000
addFullAccessKeyTokenBurnt = 42455506250000000000

# Cache data file directory, default = "", current directory: ./data
dataDir = "/home/golang/data"
```
