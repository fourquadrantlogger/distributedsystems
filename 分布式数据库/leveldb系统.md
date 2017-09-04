## level DB

1. 编码
LevelDB内部通过采用变长编码，对数据进行压缩来减少存储空间，采用CRC进行数据正确性校验。关于Varint和CRC在另一篇文章<< LevelDB SSTable格式详解>>已经进行了比较详细的论述，所以此处不再介绍。Snappy被用于SSTable的Block的压缩中。

1.1. Varint
https://developers.google.com/protocol-buffers/docs/encoding

1.2. CRC
http://en.wikipedia.org/wiki/Cyclic_redundancy_check

1.3. Snappy压缩
Snappy是Google开源的压缩库，其特点是并不追求压缩率，旨在维持一定的压缩率的情况下可以达到很高的解压缩速率，比如通过一个Core i7处理器的单个核，就可以达到250 MB/sec的压缩速率和500 MB/sec的解压速率。它被广泛应用于Google内部，其中就包括BigTable，MapReduce和RPC。非常健壮，即便遇到损坏或者恶意的输入文件都不会崩溃。

项目主页：https://code.google.com/p/snappy/。关于Snappy的一个分析：http://dirlt.com/Snappy.html

2. 多线程编程
LevelDB是一个多线程系统，会有后台线程负责sstable的compact，同时用户也可能采用多线程进行并发访问。因此需要一些机制来协调线程工作，控制memtable等共享资源的并发读写，主要通过如下三种机制来实现。在这里我们主要关注port/port_posix.h里的对应实现。
  port::Mutex mutex_;
  port::AtomicPointer shutting_down_;
  port::CondVar bg_cv_;          // Signalled when background work finishes

2.1. 互斥锁
关于pthread_mutex_t的相关知识，可参考：互斥锁pthread_mutex_t的使用。用户通过调用pthread_mutex_lock来给互斥量上锁，互斥量一旦被锁上以后，其他线程如果想给它上锁，就会阻塞在这个操作上，直到获得锁为止。得到互斥量以后就可以进入关键代码区了，操作完成之后，必须调用pthread_mutex_unlock，这样其他等待该锁的线程才有机会获得锁，否则其他线程将会永远阻塞。

在LevelDB里，采用动态方式通过调用pthread_mutex_init初始化互斥锁，mutexattr采用默认属性。主要有如下两种用法：一个是单纯的作为Mutex，另一个用法是封装在MutexLock中，在MutexLock的构造和析构函数中进行加锁和解锁。

主要有如下地方使用到了互斥锁：db/db_impl.cc 中DBImpl 有成员变量port::Mutex mutex_;此外在db/db_impl.cc，db/db_bench.cc，util/cache.cc，helpers/memenv/memenv.cc中还都使用了MutexLock。

在很多过程中都会用到互斥锁，比如：
l DBImpl析构中等待后台compaction结束的过程
l DBImpl::WriteLevel0Table中将memtable内容写入sstable时
l DBImpl::CompactMemTable中versions_->LogAndApply时
l DBImpl::CompactRange中访问versions_->current()时
l DBImpl::BackgroundCall()中
l DBImpl::OpenCompactionOutputFile中创建output file时
l DBImpl::NewInternalIterator
l DBImpl::Get
l DBImpl::GetSnapshot
l DBImpl::ReleaseSnapshot
l DBImpl::Write
l DBImpl::GetProperty
l DBImpl::GetApproximateSizes
l DB::Open
基本上所有的DB相关函数都使用到了该Mutex变量。因此理解这些函数中何时为何加锁解锁以及不加会有什么问题，对于理解函数实现是至关重要的。此外该Mutex变量还会被传入VersionSet::LogAndApply中，传入的目的主要是为了优化锁，在写入MANIFEST log时会进行解锁。

2.2. 条件变量
与互斥量不同，条件变量是用来等待而不是用来上锁的。条件变量用来自动阻塞一个线程，直到某特殊情况发生为止。通常条件变量和互斥锁同时使用。条件变量使我们可以睡眠等待某种条件出现。条件变量是利用线程间共享的全局变量进行同步的一种机制，主要包括两个动作：一个线程等待"条件变量的条件成立"而挂起；另一个线程使"条件成立"（给出条件成立信号）。

条件的检测是在互斥锁的保护下进行的。如果一个条件为假，一个线程自动阻塞，并释放等待状态改变的互斥锁。如果另一个线程改变了条件，它发信号给关联的条件变量，唤醒一个或多个等待它的线程，重新获得互斥锁，重新评价条件。

利用pthread_cond_t和Mutex实现CondVar。在LevelDB中有如下地方使用了CondVar：
port::CondVar bg_cv_;用于等待后台compaction线程的结束
port::CondVar cv;用于等待Writer完成

2.3. 原子性指针
原子变量即操作变量的操作是原子的，该操作不可再分，因此是线程安全的。使用原子变量的原因是多个线程对单个变量操作也会引起问题。原子变量只是保证单个变量在某一个操作过程的原子性，但是无法保证整个程序的安全性。当共享资源是位或整型变量，是一个完整的加锁体制对于一个简单的整数值看来过分了. 对于这样的情况，使用原子变量效率更高。

在LevelDB中使用的是AtomicPointer，用来提供对指针变量的原子性访问。有两种实现方式，一种是采用cstdatomic，一种是采用MemoryBarrier。基于MemoryBarrier的实现要比cstdatomic快很多，关于MemoryBarrier的作用可参考Why Memory Barrier，barrier内存屏蔽。

可以看到MemoryBarrier()实际上就是如下的一条汇编语句：
__asm__ __volatile__("" : : : "memory");
1）__asm__用于指示编译器在此插入汇编语句
2）__volatile__用于告诉编译器，严禁将此处的汇编语句与其它的语句重组合优化。即：原原本本按原来的样子处理这这里的汇编。
3）memory强制gcc编译器假设RAM所有内存单元均被汇编指令修改，这样cpu中的registers和cache中已缓存的内存单元中的数据将作废。cpu将不得不在需要的时候重新读取内存中的数据。这就阻止了cpu又将registers，cache中的数据用于去优化指令，而避免去访问内存。
4）"":::表示这是个空指令。

如下地方使用到了AtomicPointer：
db/skiplist.h
port::AtomicPointer next_[1];
port::AtomicPointer max_height_;
db/db_impl.h
port::AtomicPointer shutting_down_;
port::AtomicPointer has_imm_;

3. 数据结构
3.1. SkipList
SkipList是LevelDB的memtable实际采用的数据结构。关于SkipList的基本原理可以参考这篇文章：跳表SkipList。跳表具有如下的结构：

LevelDB 理论基础 - 星星 - 银河里的星星

SkipList的实现简单，可以用来替代平衡树。 LevelDB中的SkipList实现在db/skiplist.h，关于其线程安全性有如下描述：写操作需要进行额外的同步，通常可以使用Mutex，读操作需要保证在读的过程中SkipList对象不会被销毁，除此之外，读操作不需要内部的锁或同步机制。可以看到在SkipList的实现中，内部并没有使用到锁，只是用到了AtomicPointer。

即使是这样，它仍在竭力避免memory barrier的使用，所以提供了NoBarrier_Next和NoBarrier_SetNext两个访问接口。另外在Insert实现中，对于max_height_的修改并没有与并发读者的任何同步化操作，这样读者可能会读到一个新的max_height_值，但是这样不会有什么问题，因为读者通过head_要么读到的是NULL(如果Insert还未完成后续的SetNext操作)，要么是新插入的这个node，如果是NULL，它就会直接转到下一层，如果读到的是新插入的这个节点，就直接使用它就好了，因此不会引发程序的崩溃。

Acquire_Load()：void* result = rep_; MemoryBarrier();return result;
Release_Store(void* v)：MemoryBarrier();rep_ = v;
我们来分析下如果在Insert中的prev[i]->SetNext(i, x);不使用Barrier会有什么问题？如下两条语句：
x->NoBarrier_SetNext(i, prev[i]->NoBarrier_Next(i));
prev[i]->SetNext(i, x);
实际上等效于如下三句：
x->next=prev[i]->next
MemoryBarrier()
prev[i]->next=x

假设如果没有内存Barrier，可能会出现如下一种情况，另一个并发线程，可能会看到不正确的内存更新顺序，比如它可能看到prev[i]->next已经是x了，但是x的next还未被正确更新，因为x有可能是存在高速缓存中的，还未刷到内存，这样就会有问题。通过Barrier就可以保证，如果用户看到了prev[i]->next=x那么x->next也肯定已经被正确的设置了。

3.2. LSM-tree
LevelDB的核心就在于实现了LSM-tree这一结构，也就是Bigtable论文中提到的tablet server的结构，关于LSM-tree的基本知识可参考：The Log-Structured Merge-Tree(译)。LSM-tree由两种组件组成，位于内存的memtable，磁盘上的sstable，当数据插入时会被首先写入到log和memtable中，当达到一定阈值后，创建一个全新的log和memtable，新的更新会被放入到新的log和memtable，同时会在后台将旧memtable内容写入sstable，sstable之间会在后台进行compact。

Memtable需要能够支持快速的插入，查询操作，同时里面的内容应该可以快速的转换为有序的sstable。在LevelDB内部memtable是用skiplist实现的。内部实际上有两个memtable：mem_和imm_。mem_是正处于可更新状态的memtable，imm_已经被锁定处于不可更新状态，未来会被dump为sstable。

LevelDB中的LSM-tree实现更像一个多组件LSM-tree，它的每个level i可以看做是一个逻辑上的Ci组件，同一个level内的key无重叠(level 0除外)，同时level越高数据越老。这点是其独特之处。

4. 基础框架
如Sanjay所说，LevelDB虽然从原理上来看就是Bigtable中的Tablet Server，但是它的代码却与Google内部的Bigtable是独立的，也就是说并不是直接从Bigtable代码剥离出来的，而是重写的。为了使之能够独立的开源，将其所依赖的其他模块进行了替换，所以LevelDB本身包含了很多机制，可能这些机制在Bigtable内是直接使用了其他模块的源码，但是如果要独立开源，必须要重新实现而不能再依赖于其他部分，虽然有些实现可能比较简陋，但是基本上包含了一个系统所需要的各种机制，仔细研究这些机制的实现应是十分有益的。

4.1. Env
include/leveldb/env.h中包含了关于文件目录操作及其他相关函数调用的封装，比如文件锁，线程创建及启动等。通过Env类，将系统相关的底层调用和操作进行了封装。这样用户就可以通过实现自己的Env，将LevelDB架在不同的文件系统上。用户只需要实现自己的Env，然后通过Options在OpenDB时传递给它，这样这些相应的操作就会使用用户自己的实现了。具体如下：
class SlowEnv : public leveldb::Env {
    .. implementation of the Env interface ...
  };
  SlowEnv env;
  leveldb::Options options;
  options.env = &env;
  Status s = leveldb::DB::Open(options, ...);

LevelDB中的默认Env实现在util/env_posix.cc中。系统会通过Env::Default()来获取默认的Env实现。观察util/options.cc，可以看到对于Options来说，默认的env就是Env::Default()。

用户如果想要移植LevelDB到新的操作系统，替换底层文件系统，就需要提供自己的Env实现。

4.1.1. 文件系统
用户需提供三种类型的文件类：SequentialFile，RandomAccessFile，WritableFile。它们将被用于支持sstable和日志文件的读写。

具体的关于SequentialFile和RandomAccessFile的Read接口中，都有一个char* scratch参数，这与我们通常见到的文件的read接口定义很不一样。同时再加上Slice* result，就给人一种重复的感觉，因为用户完全可以传一个Slice* result，令result.data指向存放数据的缓冲区。但是实际上这两个参数承担了不同的角色， scratch参数所指向的缓冲区是由外部提供的，而不是Read函数内部new出来的，result则是用来保存数据最终存放的位置。

在实现一个SequentialFile和RandomAccessFile的子类时，实现者可以有两种选择：可以直接利用scratch传递保存读取数据的缓冲区的地址，也可以利用自己内部new的或者某个已知的地址存放数据，利用result直接传递最终存放地址。所以include/leveldb/env.h中的注释上说“"scratch[0..n-1]" may be written by this routine.”，也就是说用户也可能不使用scratch。新加的PosixMmapReadableFile就利用了这一点，可以看到它的Read函数并没有利用scratch来存放数据。

当然最简单的就是，用户在自己的File实现中直接选择scratch作为存储地址，不用担心出错。如果自己在Read中去new新的内存，需要自己负责管理好内存，避免内存泄露。

4.1.2. Sync()与Flush()
此外，需要注意的是，可以看到env.h中的WritableFile有两个接口Sync和Flush。观察util/env_posix.cc中实现，可以看到Flush()只提供了一个空的实现，而Sync()内调用了fdatasync，msync。而再观察整个LevelDB，可以发现有两个地方会调用WritableFile的Flush函数：一个是在table/table_builder.cc中写入内容达到定义的block size时会调用一次Flush()；另一个是在db/log_writer.cc中，每输出一条物理记录都会进行一次Flush()调用。

而Sync()调用，通过查看include/leveldb/options.h，可以看到实际上在WriteOptions有一个选项sync用来控制每次write操作是否调用Sync(){!通过后面可知实际上它控制是每次write的操作日志写入部分是否进行Sync}。而Sync的语义是将数据从操作系统的buffer cache中flush到磁盘。在LevelDB中可以看到有如下地方调用了Sync()：db/builder.cc中写完一个sstable的后会调用一次Sync()；DBImpl::Write中在WriteOptions打开sync选项的情况下，会对每次log的写入都调用Sync(){!我们知道每次写入实际上是先写log，然后再写入memtable}；同时在每次compaction结束后，因为都要生成一个新的sstable文件，此时也会调用Sync()。

所以，Sync是相对更重量级的一种操作，而Flush应该通常是在客户端文件操作接口内部实现了缓存机制时可能才需要提供，通常情况下提供一个空的实现也是可以的。当然只要知道了它们各自的调用时机，用户就可以自行决定如何实现自己的Sync和Flush。

此外我们可以看到在linux上也有fflush()和fsync()两个调用，fflush是一个c库函数，负责将c中write函数调用后的缓冲区内容写回磁盘，实际上是写回内核缓冲区。而fsync()则是一个系统调用，负责把内核缓冲区刷到磁盘。另外HDFS中也有hflush和hsync两个函数，根据注释可知hflush负责刷出客户端的用户缓冲区，hsync则类似于posix fsync，将用户缓冲区内容刷到磁盘设备，当然磁盘可能把这些内容先放到自己的cache中。

4.2. Port
涉及到移植性的各方面。因为不同的系统上，这些特性的实现方式，是否支持都可能是不同的，因此必须将其封装以提供针对这些系统的实现。

4.2.1. port_example.h
如何将LevelDB移植到其他系统上的一个例子。主要包含：大小端，线程控制(Mutex CondVar AtomicPointer)，Snappy_Compress，GetHeapProfile。

4.2.2. port_posix.cc
针对posix系统的CondVar条件变量 ，Mutex的实现及Snappy压缩解压缩实现。

4.3. 迭代器框架
对于LevelDB来说，迭代器是数据的读取的核心。从KeyValue组成的Block，Block组成的sstable，sstable组成的level，level组成的version，到最上层的db，都是通过iterator组织的。

LevelDB 理论基础 - 星星 - 银河里的星星

同时DB merge Iterator还封装了snapshot相关的过滤，及将internalkey转换为uerkey的工作。通过这样一个多层次的iterator结构，向用户提供了可以访问底层keyvalue的接口，隐藏掉了中间的大量细节，用户直接看到的就是一系列连续的keyvalue。

4.3.1. merge.cc

对多个迭代器进行merge，总体上比较简单，需要注意的是目前求最小和最大值的方法FindSmallest和FindLargest采用的都是直接遍历所有的子迭代器，没有使用堆来实现。

另外Next()和Prev()操作中，都有一段比较长的判断用于将其他非current子迭代器定位到>=key()的位置。这是为了防止用户穿插调用Next()和Prev()时，可能造成的语义混乱。比如假设current=a;调用Next后current=b;之后又调用了Prev;current=c。如果不进行这种对齐操作，可能会出现c>b的情况，这就与Prev的语义相矛盾了。

4.3.2. two_level_iterator.cc

一个两级迭代器。第一级针对table中的index_block的block handle列表，第二级则是block内部的key value迭代器。封装了block的切换等操作。用户可以直接使用Next()实现对整个sstable的key value对的遍历。

4.4. 内存分配器

util/arena.cc

内存分配器，申请的内存单元通常是4KB大小的内存块，但是当用户某次申请空间大于1KB时且无法存放到剩余内存时，该分配器会直接向系统申请用户所需大小的内存。这主要是避免这种大块内存产生太多内存空洞。因为如果它不是去申请一个刚好等于该内存块大小的新块，而还是需要申请固定的4KB大小，一方面是有可能依然放不下，另一方面是它会引起alloc_ptr_指针的改变，这样上一个未用完的4KB块就不能再用了。

此外，通过Arena申请的内存，只能在Arena对象析构时才会释放。在LevelDB中只是memtable用它来做内存申请，具体来说就是会用于SkipList的Node分配。Memtable内含Arena和Table两个成员变量，所以可知当Memtable析构的时候，其对应的Arena成员申请的内存也会释放掉。因此关键看Memtable何时会被析构，实际上在达到内存上限后，Memtable会由mem_变为imm_，而imm_会在DBImpl::CompactMemTable()中被dump成sstable，dump完成后，会调用：
imm_->Unref();
imm_ = NULL;
has_imm_.Release_Store(NULL);
而在Unref中，当当前对象的引用计数<=0时，会调用delete this;
所以说Memtable通过Arena所占用的内存，就是这样释放的。由于Memtable本身大小有个限制，因此Arena所申请的内存也会是有限的，所以只是在它析构的时候释放它已经申请的内存也是可以接受的。

4.5. Cache
util/cache.cc实现了一个LRU cache，该cache会用来缓存sstable中的block，打开的sstable文件及对应的Table对象。

memtable：memtable实际上充当了写入数据的cache，而Option中的一个参数write_buffer_size就是用来控制memtable大小的。当memtable接近write_buffer_size大小时，就会进行切换到imm_或者阻塞写操作。具体控制是在DBImpl:: MakeRoomForWrite中进行的。Memtable底层采用skiplist实现，它内部会使用一个自己实现的名为Arena的块内存分配器为分配skplist节点。

Block cache：会对每个sstable的block进行缓存。内部采用了util下的cache.cc里的LRUCache实现。对应的cache entry中，以table对应的cache_id+block在table内的offset为key，以Block*为value。

Table cache：db/ table_cache.cc。TableCache是用于缓存打开的sstable文件及对应的Table对象的。其内部与block cache一样采用了util下的cache.cc里的LRUCache实现。对应的cache entry中，以sstable文件的file_number为key，以TableAndFile*为value。table_cache实际上是对已经打开的sstable文件及相关的Table*结构进行了缓存。

4.6. Snapshot
LevelDB支持Snapshot，用户需要首先创建某一个时刻的Snapshot，之后在读取时设置ReadOptions::snapshot就可以读取之前那个时刻的Snapshot，实际上通过Snapshot维护了关于整个key-value存储状态的一致性的只读视图。

Snapshot本身很简单，只是简单地将它与一个sequence number关联，然后系统只需要保证不对大于等于该snapshot所对应的sequence number的那些记录进行compact(防止delete标记清掉老的keyvalue)，然后在读取时跳过大于该sequence number的所有记录，就可以读出该snapshot记录的状态。而所有的snapshots又是通过一个双向链表维护的。所以GetSnapshot和ReleaseSnapshot，都是很轻量级的操作，本质上只是返回了一下sequence number，及将其插入或删除snapshots的双向链表。同时用户只能通过DB::GetSnapshot()或者是通过设置write_options.post_write_snapshot来创建snapshot，所以snapshot的创建都是直接获取调用时的sequence number插入到snapshots链表中，也就是说调用本身已保证了sequence number是严格按照调用递增的，所以snapshots按照插入顺序就是有序的，不需要额外的排序。

snapshot对应的状态实际上用一个SequenceNumber就足以描述了。
db_impl.cc中struct DBImpl::CompactionState中有个变量：
  // Sequence numbers < smallest_snapshot are not significant since we
  // will never have to service a snapshot below smallest_snapshot.
  // Therefore if we have seen a sequence number S <= smallest_snapshot,
  // we can drop all entries for the same key with sequence numbers < S.
  SequenceNumber smallest_snapshot;
sequence number会被记录在versions_中。每次用户执行一个更新操作时，它都会+1，所以说sequence number在这里体现的不是操作发生的时间，而是对操作的计数，当然sequence number越大也说明操作越新。同时观察db_impl.cc 中DBImpl::Write实现可以看出，系统首先将更新通过log_->AddRecord写入了日志后，才通过WriteBatchInternal::InsertInto将这些更新插入到memtable中，也就是说采用了WAL机制。

sequence number是在何处被+1的呢？无论是DBImpl::Put还是DBImpl::Delete，实际上最终都是通过DBImpl::Write实现的，而它内部用于更新memtable的是WriteBatchInternal::InsertInto，而InsertInto 内部是通过MemTableInserter完成插入的，而观察write_batch.cc的MemTableInserter实现可知，它的每次Put和Delete都自动sequence_++，最终我们找到了写入到DB里的记录的sequence进行递增的地方。同时DBImpl::Write本身会更新versions_里记录的last_sequence为最新状态，last_sequence就变成了最新的sequence number。

4.7. Version
实际上除snapshot的概念外。DB还有一个与之类似的概念version。如果说snapshot是记录级别的快照的话，version则可以看做是文件级别的视图。下面我们需要搞清楚迭代器是如何与Version协同进行工作的呢？compaction过程中Versionset以及snapshots又会起到什么作用呢？

Version与versionset：Version实际上是当前各level下的文件的一个视图，在创建一个iterator时，就会增加对当前Version的引用计数，而Version会通过它内部的一个具有config::kNumLevels个std::vector<FileMetaData*>元素的数组来记录当时各个level下的文件。同时每个version还会记下用户读取记录时的一些统计信息，这些信息会用来帮助判断是否启动compaction。

Versionset 之所以存在，主要是为了给各个迭代器提供一个一致性的视图，即当打开一个迭代器时，就会记录一下当前的Version，这样就可以避免comact等过程文件的合并删除影响到当前的读者，由于可能存在多个迭代器，因此会产生多个Version，这多个Version就是通过VersionSet管理的。

Versionset除了会管理一系列现有version外，还有一个很重要的变量Version* current_，通过不断地在current_的version上进行LogAndApply对其进行更新，最新的文件视图信息会被保存在CURRENT文件所指定的manifest文件中。此外它还包含了其他文件当前的序列号。同时Versionset还有一个compact_pointer_数组，用来记住每个level上次compact截止的key，下一次就会继续从该key开始进行compact，如此循环往复。

VersionEdit则记录相对于某个基准Version的状态变化，比如：正在进行的compaction的key起点，删除了某些文件，增加了某些新文件。

4.7.1. Version与versionset管理

数据结构定义

首先每个Version具有如下数据成员：
  VersionSet* vset_;            // VersionSet to which this Version belongs
  Version* next_;               // Next version in linked list
  Version* prev_;               // Previous version in linked list
  int refs_;                    // Number of live refs to this version

  // List of files per level
  std::vector<FileMetaData*> files_[config::kNumLevels];

  // Next file to compact based on seek stats.
  FileMetaData* file_to_compact_;
  int file_to_compact_level_;

  // Level that should be compacted next and its compaction score.
  // Score < 1 means compaction is not strictly needed.  These fields
  // are initialized by Finalize().
  double compaction_score_;
  int compaction_level_;
其中包含了指向前一个和后一个Version，以及它所属的VersionSet的指针，对该Version的引用计数，记录着各个level下的文件组成。

VersionSet具有如下数据成员：
  Env* const env_;
  const std::string dbname_;
  const Options* const options_;
  TableCache* const table_cache_;
  const InternalKeyComparator icmp_;
  uint64_t next_file_number_;
  uint64_t manifest_file_number_;
  uint64_t last_sequence_;
  uint64_t log_number_;
  uint64_t prev_log_number_; // 0 or backing store for memtable being compacted

  // Opened lazily
  WritableFile* descriptor_file_;
  log::Writer* descriptor_log_;
  Version dummy_versions_;  // Head of circular doubly-linked list of versions.
  Version* current_;        // == dummy_versions_.prev_

  // Per-level key at which the next compaction at that level should start.
  // Either an empty string, or a valid InternalKey.
  std::string compact_pointer_[config::kNumLevels];


当前Version的更新过程

比如在调用DBImpl::CompactMemTable()后就会产生一个新的sstable，导致当前文件视图发生变化，versions_通过current维护最新的视图，在视图变化后，需要将描述Version变化的VersionEdit进行LogAndApply(VersionEdit* edit)，这样这些变更就会记录到日志同时更新最新的Version状态，更新完成后会被append到现有versions的链表中，。具体来说，version_set.cc中的LogAndApply函数通过
  Version* v = new Version(this);
  {
    Builder builder(this, current_);
    builder.Apply(edit);
    builder.SaveTo(v);
  }
  Finalize(v);
将VersionEdit应用到当前的Version上，产生出一个新的Version V，然后通过AppendVersion将该Version加入到现有versions的链表，并将current_指向该最新的version。

同时可以看到，在DBImpl::BackgroundCompaction()中也会有相关的动作，也就是说在文件视图发生变化后，都会通过该调用来保证进行文件视图的更新，。而在创建Iterator时DBImpl::NewInternalIterator，则会增加对当前version的引用计数。通过引用计数来保证不会破坏那些正被某些Iterator引用的那些version。

Version VersionSet与compaction

当前有人在读，那么memtable会不会被dump，还是会被锁定？如果创建迭代器时也创建了一个Version，而该Version只包含了sstable文件，并未包含memtable，那么比如当Version创建之后，memtable又发生了dump，而此时dump出去的sstable不在version之内，但是merge读的时候又读取了最新的memtable，那么就意味着有中间产生的一部分sstable内容是被落下了，按照语义这肯定是不正常的？那么内部读的迭代器，version与memtable，sstable之间到底是怎么组织的呢？

VersionSet有个成员函数：Compaction* PickCompaction();该函数实际决定了到底对哪个level，以及该level下的那些文件进行compaction。

每个version本身有一个引用计数，同时它所记录的各个level下的文件对应的FileMetaData信息，也有一个引用计数。

4.7.2. version_edit.cc
version_edit.cc对应于MANIFEST文件。它会为每个sstable保存它的level，文件序列号，文件大小，最小key，最大key。

4.7.3. version_set.cc
version_set.cc应该是对DB中处于各个level下的sstable文件视图版本进行管理。因为DB中的sstable文件因为compaction的存在一直处于不断变化中。同时还有用户迭代器在引用着现有的某些sstable。而LevelDB正是通过version+引用计数的概念来维持一个一致性的文件视图，避免compaction对迭代器的影响。

每次文件视图发生变更，都会导致新的version的产生，除维护这样一个最新的文件视图外，同时只要还有引用对应于相应的version，LevelDB就会维护着之前的这个version。而最新各个level下的sstable文件视图还会被保存到MANIFEST文件中。而MANIFEST，实际上有两个文件与之相关，一个是记录文件视图信息的descriptor_file_，另一个则是记录MANIFEST变更记录的descriptor_log_。

该文件中还有一个函数：static uint64_t MaxFileSizeForLevel(int level)
该函数实际上是用来控制各个level下的文件大小的，目前的默认每个level下的文件大小一样，都是kTargetFileSize，即2MB，实际上用户可以修改它来让不同level下的文件具有不同的大小限制，这样可以控制level的层数。

4.8. memenv
提供了一个完全基于内存的文件系统接口。这对于单元测试和性能分析是很有用的，使用memenv，就无需在单元测试中去做文件的创建和删除，同时性能分析中也可以去除磁盘的影响，可以方便地了解到各种操作在CPU方面的性能，同时也可以与磁盘文件做性能对比分析以了解IO方面的开销。通过这个InMemoryEnv，用户就可以将LevelDB架在内存中。

4.9. 原子写
LevelDB支持将一系列更新操作累积在一块，作为一个单一的原子性操作执行。通过leveldb::WriteBatch将一系列更新操作保存到一块，然后调用DBImpl::Write将一系列更新实际写入。

4.10. 单元测试框架
为支持单元测试，首先util/testutil.h里提供了一些工具函数包括：
RandomString：用于随机生成给定长度的字符串
RandomKey：用于随机生成给定长度的key，与RandomString的区别在于可选字符集不同
CompressibleString：用于生成一系列可压缩字符串，同时考虑压缩率这个因素，先根据压缩后的长度随机生成好一个串，然后根据压缩率将该串重复，以达到压缩前的大小，这样该串就比较容易能达到给定的压缩率
ErrorEnv：用于模拟底层IO错误

此外，util/testharness.h里则实现了一个简单的test框架。用户通过Tester执行各种assert操作，如果assert失败，会在Tester析构时打印出出错信息并退出；通过RegisterTest注册用户测试函数，该函数会将用户测试函数信息，保存到一个全局变量tests中；然后RunAllTests运行注册的用户测试函数。基本上包含了一个测试框架所需的基本功能。

4.11. 错误与异常处理机制
LevelDB通过定义了一个Status类来封装各种返回值信息。如果单纯的采用bool值的话，只能表示成功，失败两种状态，而如果采用int作为返回值又缺乏表现力，无法描述具体错误，所以LevelDB定义了一个Status类。通过Status来标识各种状态，使得用户可以清楚的系统发生了何种错误，也可以更好地做出响应，同时效率也不会降低。

4.12. 日志系统
LevelDB还实现了自己的日志系统。util/posix_logger.h
这里实现的logger还比较简单，没有分级别，只是简单地将信息记录下来。

5. 代码风格
· 随处可见的变长编码，内存pack和前缀压缩
· 遍布全局的assert
· 能使用前置声明的尽量使用，减少编译依赖
· 注释清晰扼要
· 文件变量命名简单明了
· 非常注重性能
· 各种cache：table cache block cache
· 隔离底层系统依赖关系 可移植性：使用Env及Port模块实现了与底层系统的分离
· 简单设计原则：比如利用sequence number实现snapshot