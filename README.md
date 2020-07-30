### 以不动产登记业务为例，使用超级账本搭建政务数据区块链原型

- ### 关于业务流程
- 联盟链网络
- 业务流程时序
- ### 关于超级账本
关于区块链、超级账本、智能合约等概念，建议直接参考官网文档 https://hyperledger-fabric.readthedocs.io/en/release-1.4/ 

- ### 关于本例说明
- 本例仅限搭建原型用于验证技术可行性
- 本例基于Fabric1.4.7版本，fabric-sdk-go@v1.0.0-beta2
- 本例初始化了3个组织，每组织2个节点，用于不动产、房管、税务三个部门做背书节点；3份智能合约（网签合同、纳税凭证、不动产权证书）跑在1个通道；共识机制采用Raft，状态数据库采用CouchDB;
- 启动fabric网络
```
cd app/
./startNetwork.sh

```
- 编译运行client
```
cd app/fcc-client
yarn install
yarn run serve
```
- 访问WEB客户端
http://localhost:8080/

- ### 需要完善的几个问题
- 证书与权限：通过CA集群和MSP来管理发放不同权限的证书；
- 国密算法支持；
- 动态添加组织和节点；

