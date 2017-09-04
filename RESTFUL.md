# RESTful架构有一些典型的设计误区。

最常见的一种设计错误，就是URI包含动词。因为"资源"表示一种实体，所以应该是名词，URI不应该有动词，动词应该放在HTTP协议中。
举例来说，某个URI是/posts/show/1，其中show是动词，这个URI就设计错了，正确的写法应该是/posts/1，然后用GET方法表示show。
如果某些动作是HTTP动词表示不了的，你就应该把动作做成一种资源。比如网上汇款，从账户1向账户2汇款500元，错误的URI是：
　　POST /accounts/1/transfer/500/to/2
正确的写法是把动词transfer改成名词transaction，资源不能是动词，但是可以是一种服务：
　　POST /transaction HTTP/1.1
　　Host: 127.0.0.1
　　
　　from=1&to=2&amount=500.00

另一个设计误区，就是在URI中加入版本号：
　　http://www.example.com/app/1.0/foo
　　http://www.example.com/app/1.1/foo
　　http://www.example.com/app/2.0/foo
因为不同的版本，可以理解成同一种资源的不同表现形式，所以应该采用同一个URI。版本号可以在HTTP请求头信息的Accept字段中进行区分（参见Versioning REST Services）：
　　Accept: vnd.example-com.foo+json; version=1.0
　　Accept: vnd.example-com.foo+json; version=1.1