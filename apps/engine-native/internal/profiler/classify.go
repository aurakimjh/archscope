// ─────────────────────────────────────────────────────────────────────
// [한글] classify — stack frame 들을 의미 카테고리로 분류 (도메인 휴리스틱).
//
// 책임/목적
//   collapsed stack 또는 frame 시퀀스를 받아 "이 stack 은 SQL 인가, HTTP
//   인가, lock 대기인가..." 를 결정. 이 분류 결과가 breakdown / timeline
//   의 그룹핑 키가 되므로 profiler UI 의 모든 표/차트의 의미 라벨이 여기서 결정.
//
// 분류 카테고리 (PrimaryCategory)
//   - LOGGING                   : SLF4J / Logback / Log4j / java.util.logging 등
//   - SQL_DATABASE              : JDBC / Oracle / java.sql / executeQuery 등
//   - EXTERNAL_API_HTTP         : RestTemplate / WebClient / OkHttp / urlconnection
//   - CONNECTION_POOL_WAIT      : Hikari / ConcurrentBag / SynchronousQueue
//   - LOCK_SYNCHRONIZATION_WAIT : LockSupport.park / Object.wait / Future.get
//   - NETWORK_IO_WAIT           : SocketRead / NioSocketImpl / netpoll / epollwait
//   - FILE_IO                   : FileInputStream / FileChannel / RandomAccessFile
//   - GC_JVM_RUNTIME            : G1 / Shenandoah / ZGC / safepoint / GC
//   - FRAMEWORK_MIDDLEWARE      : Spring boot startup, batch job runner 등
//   - APPLICATION_LOGIC         : com./org./service/controller 등 일반 코드
//   - UNKNOWN                   : 그 외
//
// 매칭 우선순위
//   Logging → SQL → HTTP → Pool → Lock → Network → File → GC → Startup → Framework → Internal → Unknown.
//   (앞에서 매치되면 뒤는 보지 않음). SQL/HTTP 는 "+ Network" 가 있으면
//   WaitReason="NETWORK_IO_WAIT" 보조 라벨을 부착해 timeline 이 "쿼리 실행
//   vs DB 응답 대기" 같은 세분화를 할 수 있게 한다.
//
// 트리키한 부분
//   • 모든 매칭은 stack 을 ";" 로 join → ToLower 한 다음 substring 검색.
//     case-insensitive 매칭이며 path 어디든 해당 token 이 등장하면 true.
//   • token 리스트는 도메인 경험 기반의 휴리스틱이라 자주 업데이트됨.
//     Python 측과 byte 단위로 동일해야 분류 결과 parity 가 보장된다.
//   • classifyStack 은 PrimaryCategory → user-facing label 매핑 (영문).
//     classifyFrames 는 PrimaryCategory + WaitReason + Label 모두 채움.
// ─────────────────────────────────────────────────────────────────────

package profiler

import "strings"

var loggingFrameTokens = []string{
	"org.slf4j",
	"ch.qos.logback",
	"org.apache.logging.log4j",
	"org.apache.log4j",
	"java.util.logging",
	"org.apache.commons.logging",
	"org.jboss.logging",
	"kotlin.logging",
	"mu.kotlin",
	"go.uber.org/zap",
	"sirupsen/logrus",
	"log/slog",
	"zerolog",
	"winston",
	"pino.",
	"loggerfactory",
	"loggingevent",
	"appenderskeleton",
	"appenderbase",
	"asyncappender",
	"fileappender",
	"consoleappender",
	"rollingfileappender",
	"logback",
	"log4j",
	"slf4j",
	".logger.",
	"logger.",
	"logger_",
	"logging.",
	".logging",
}

var frameworkFrameTokens = []string{
	"org.springframework.",
	"springframework",
	"dispatcherservlet",
	"frameworkservlet",
	"invocablehandlermethod",
	"requestmappinghandleradapter",
	"handleradapter",
	"handlerinterceptor",
	"transactioninterceptor",
	"filterchainproxy",
	"onceperrequestfilter",
	"springsecurity",
	"springbatch",
	"org.aopalliance",
	"org.aspectj",
	"cglib",
	"jakarta.servlet",
	"javax.servlet",
	"applicationfilterchain",
	"org.apache.catalina",
	"org.apache.coyote",
	"org.apache.tomcat",
	"standardwrappervalve",
	"http11processor",
	"io.undertow",
	"org.eclipse.jetty",
	"org.glassfish.jersey",
	"org.jboss.resteasy",
	"org.quartz",
	"quartzscheduler",
}

// [한글] stackClassification — 분류 결과 묶음.
// PrimaryCategory: 메인 카테고리(영문 enum string), WaitReason: SQL/HTTP +
// Network 같은 보조 분류, Label: UI 표시용 영문 라벨.
type stackClassification struct {
	PrimaryCategory string
	WaitReason      *string
	Label           string
}

func classifyStack(stack string) string {
	classification := classifyFrames(splitStack(stack))
	switch classification.PrimaryCategory {
	case "LOGGING":
		return "Logging"
	case "SQL_DATABASE":
		return "Database"
	case "EXTERNAL_API_HTTP":
		return "External API"
	case "CONNECTION_POOL_WAIT":
		return "Connection Pool"
	case "LOCK_SYNCHRONIZATION_WAIT":
		return "Lock / Sync"
	case "NETWORK_IO_WAIT":
		return "Network / I/O"
	case "FILE_IO":
		return "File I/O"
	case "GC_JVM_RUNTIME":
		return "JVM / GC"
	case "FRAMEWORK_MIDDLEWARE":
		return "Framework"
	case "APPLICATION_LOGIC":
		return "Application"
	default:
		return "Other"
	}
}

// [한글] classifyFrames — 핵심 분류 함수.
// 모든 frame 을 join 후 lower 하여 substring 매칭으로 카테고리 후보를
// 동시에 평가. 우선순위: Logging → SQL → HTTP → Pool → Lock → Network →
// File → GC → Startup → Framework → Internal → Unknown. SQL/HTTP 는 Network 가 함께 있으면
// WaitReason="NETWORK_IO_WAIT" 보조 분류 부착.
func classifyFrames(path []string) stackClassification {
	stack := strings.ToLower(strings.Join(path, ";"))
	hasLogging := containsAny(stack, loggingFrameTokens...)
	hasSQL := containsAny(stack, "oracle.jdbc", "java.sql", "t4cpreparedstatement", "t4cmarengine", "executequery", "executeupdate", "resultset", "org.hibernate", "jakarta.persistence", "javax.persistence", "entitymanager", "org.mybatis", "org.apache.ibatis", "mybatis", "ibatis", "defaultsqlsession", "mappermethod", "preparedstatementhandler", "simpleexecutor", "cachingexecutor", "org.jooq")
	hasHTTP := containsAny(stack, "resttemplate", "webclient", "httpclient", "okhttp", "urlconnection", "mainclientexec", "bhttpconnection", "openfeign", "feign.", "reactor.netty.http.client", "java.net.http", "jdk.internal.net.http", "grpc.")
	hasNetwork := containsAny(stack, "socketinputstream.socketread", "niosocketimpl", "socketread", "socket.read", "netpoll", "epollwait")
	hasPool := containsAny(stack, "hikaripool.getconnection", "hikari.pool", "concurrentbag", "synchronousqueue", "tomcat.jdbc.pool", "commons.dbcp", "c3p0")
	hasLock := containsAny(stack, "locksupport.park", "unsafe.park", "object.wait", "future.get", "monitor.enter", "mutex.lock")
	hasFile := containsAny(stack, "fileinputstream", "filechannel", "randomaccessfile", "bufferedreader.readline")
	hasGC := containsAny(stack, "g1youngcollector", "shenandoah", "zgc", "safepoint", "garbagecollect")

	if hasLogging {
		return stackClassification{PrimaryCategory: "LOGGING", Label: "Logging"}
	}
	if hasSQL {
		if hasNetwork {
			return stackClassification{PrimaryCategory: "SQL_DATABASE", WaitReason: stringPtr("NETWORK_IO_WAIT"), Label: "SQL database"}
		}
		return stackClassification{PrimaryCategory: "SQL_DATABASE", Label: "SQL database"}
	}
	if hasHTTP {
		if hasNetwork {
			return stackClassification{PrimaryCategory: "EXTERNAL_API_HTTP", WaitReason: stringPtr("NETWORK_IO_WAIT"), Label: "External API"}
		}
		return stackClassification{PrimaryCategory: "EXTERNAL_API_HTTP", Label: "External API"}
	}
	if hasPool {
		return stackClassification{PrimaryCategory: "CONNECTION_POOL_WAIT", Label: "Connection pool wait"}
	}
	if hasLock {
		return stackClassification{PrimaryCategory: "LOCK_SYNCHRONIZATION_WAIT", Label: "Lock / synchronization wait"}
	}
	if hasNetwork {
		return stackClassification{PrimaryCategory: "NETWORK_IO_WAIT", Label: "Network / I/O wait"}
	}
	if hasFile {
		return stackClassification{PrimaryCategory: "FILE_IO", Label: "File I/O"}
	}
	if hasGC {
		return stackClassification{PrimaryCategory: "GC_JVM_RUNTIME", Label: "JVM / GC runtime"}
	}
	if looksLikeStartup(path) {
		return stackClassification{PrimaryCategory: "FRAMEWORK_MIDDLEWARE", Label: "Framework"}
	}
	if looksLikeFrameworkMiddleware(path) {
		return stackClassification{PrimaryCategory: "FRAMEWORK_MIDDLEWARE", Label: "Framework / middleware"}
	}
	if looksLikeInternal(path) {
		return stackClassification{PrimaryCategory: "APPLICATION_LOGIC", Label: "Application logic"}
	}
	return stackClassification{PrimaryCategory: "UNKNOWN", Label: "Others"}
}

func containsAny(value string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func looksLikeStartup(path []string) bool {
	stack := strings.ToLower(strings.Join(path, ";"))
	return containsAny(
		stack,
		"springapplication.run",
		"joblauncher",
		"commandlinejobrunner",
		"simplejoblauncher",
		"batchapplication",
		"main(",
		".main",
		"application.run",
	)
}

func looksLikeFrameworkMiddleware(path []string) bool {
	if len(path) == 0 {
		return false
	}
	leaf := strings.ToLower(path[len(path)-1])
	return containsAny(leaf, frameworkFrameTokens...)
}

func looksLikeInternal(path []string) bool {
	stack := strings.ToLower(strings.Join(path, ";"))
	return containsAny(stack, "com.", "org.", "service", "controller", "processor", "writer", "reader", "tasklet", "job")
}
