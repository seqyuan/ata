<p align="center">
  <img alt="GoReleaser Logo" src="https://avatars2.githubusercontent.com/u/24697112?v=3&s=200" height="140" />
  <h3 align="center">parta</h3>
  <p align="center">A go binaries for parrellel task.</p>
</p>

---

# Installation

## Using go install (Recommended)
```bash
go install github.com/seqyuan/parta/cmd/parta@latest
```

This installs `parta` to your `$GOPATH/bin` or `$GOBIN` directory.

## From source
```bash
git clone https://github.com/seqyuan/parta.git
cd parta
go build -o parta ./cmd/parta
```

# parta
parrellel task, 任务多线程工具

# 程序功能
> 程序适用于有很多运行时间短，但是需要运行很多的脚本，有助于减少投递的脚本。
> 例如有1000个cat 命令需要执行，这些命令间没有依赖关系，每个cat命令运行在2min左右

1. 在一个进程里并行的执行指定的命令行
2. 并行的线程可指定
3. 如果并行执行的其中某些子进程错误退出，再次执行此程序的命令可跳过成功完成的项只执行失败的子进程
4. 所有并行执行的子进程相互独立，互不影响
5. 如果并行执行的任意一个子进程退出码非0，最终parta 也是非0退出
6. parta会统计成功运行子脚本数量以及运行失败子脚本数量输出到stdout，如果有运行失败的脚本会输出到parta的stderr

# 使用方法

## 程序参数
```
-i  --infile  Work.sh, same as qsub_sge's input format
-l  --line    Number of lines as a unit. Default: 1
-t  --thread   Thread process at same time. Default: 1
```

## 使用示例

`parta -i input.sh -l 2 -t 2`

标准错物流的输出：

```
[1 2 3 4 5]
All works: 5
Successed: 3
Error: 2
Err Shells:
2	/Volumes/RD/parrell_task/input.sh.shell/work_000002.sh
3	/Volumes/RD/parrell_task/input.sh.shell/work_000003.sh
```

运行产生的目录结构：
```
.
├── input.sh
├── input.sh.db
└── input.sh.shell
    ├── work_000001.sh
    ├── work_000001.sh.e
    ├── work_000001.sh.o
    ├── work_000001.sh.sign
    ├── work_000002.sh
    ├── work_000002.sh.e
    ├── work_000002.sh.o
    ├── work_000003.sh
    ├── work_000003.sh.e
    ├── work_000003.sh.o
    ├── work_000004.sh
    ├── work_000004.sh.e
    ├── work_000004.sh.o
    ├── work_000004.sh.sign
    ├── work_000005.sh
    ├── work_000005.sh.e
    ├── work_000005.sh.o
    └── work_000005.sh.sign
```

### -i
`-i` 参数为一个shell脚本，例如`input.sh`这个shell脚本的内容示例如下
```
echo 1
echo 11
echo 2
sddf
echo 3
grep -h
echo 4
echo 44
echo 5
echo 6
```

### -l
依照`-i`参数的示例，一共有10行命令，比如我们想每2行作为1个单位并行的执行，那么`-l`参数设置为2

### -t
如果要对整个parta程序所在进程的资源做限制,可设置`-t`参数,指定最多同时并行多少个子进程

### parta产生的文件

1. `input.sh.db`文件，此文件为sqlite数据库
2. `input.sh.shell`目录，`prefix`即为`-i`参数的值，例如-i参数为work.sh，则产生work.sh,shell目录
3. 按照`-l`参数切割的input.sh的子脚本，存放在`input.sh.shell`目录，以**work_000**作为子脚本的前缀，例如`-l`参数为3，则把input.sh从第一行命令开始，每3行写入到work_000前缀命名的子脚本中


### 其他使用方式
`parta -i input.sh -l 2 -t 2`

我们可以把以上命令写入到`work.sh`里，然后把`work.sh`投递到SGE或者K8s计算节点

# input.sh.db数据库
parta会针对每一个输入脚本，在脚本所在目录生成`脚本名称`+`.db`的sqlite3数据库，用于记录各`子脚本`的运行状态，例如`input.sh`对应的数据库名称为`input.sh.db`

`input.sh.db`这个sqlite3数据库有1个名为`job`的table，`job`主要包含以下几列

```
0|Id|INTEGER|1||1
1|subJob_num|INTEGER|1||0
2|shellPath|TEXT|0||0
3|status|TEXT|0||0
4|exitCode|integer|0||0
5|retry|integer|0||0
6|starttime|datetime|0||0
7|endtime|datetime|0||0
```
*  **subJob_num** 列表示记录的是第几个子脚本
*  **shellPath**为对应子脚本路径
*  **status**表示对应子脚本的状态，状态有4种: Pending Failed Running Finished
*  **exitCode**为对应子脚本的退出码
*  **retry**为如果子脚本出错的情况下parta程序自动重新尝试运行该出错子进程的次数（目前还未启用此功能）
*  **starttime**为子脚本开始运行的时间
*  **endtime**为子脚本结束运行的时间

# QA

### 并行子进程中其中有些子进程出错怎么办？
例如示例所示`input.sh`中的第2个和第3个子脚本出错，那么待`input.sh`退出后，修正子脚本的命令行，再重新运行或者投递`input.sh`即可。在重新运行
`work.sh`时，parta会自动跳过已经成功完成的子脚本，只运行出错的子脚本。

### 要在alpine docker中运行怎么办
> alpine 镜像默认不带sqlite3，parta依赖于sqlite3，alpine需要更新，Dockerfile可以参考下面的

```
FROM alpine:latest

MAINTAINER Yuan Zan <seqyuan@gmail.com>

COPY ./parta /opt/
WORKDIR /opt

RUN apk update && apk add --no-cache \
	ttf-dejavu sqlite bash && \
	mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2 && \
	chmod +x parta
	if [ -e /bin/sh ];then rm /bin/sh ; fi \
	&& if [ -e /bin/bash ];then ln -s /bin/bash /bin/sh ; fi
	
ENV PATH /opt:$PATH:/bin
```

这样就能直接在docker容器内的命令行使用parta，而不必写绝对路径了