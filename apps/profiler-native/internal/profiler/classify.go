package profiler

import "strings"

type stackClassification struct {
	PrimaryCategory string
	WaitReason      *string
	Label           string
}

func classifyStack(stack string) string {
	classification := classifyFrames(splitStack(stack))
	switch classification.PrimaryCategory {
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

func classifyFrames(path []string) stackClassification {
	stack := strings.ToLower(strings.Join(path, ";"))
	hasSQL := containsAny(stack, "oracle.jdbc", "java.sql", "t4cpreparedstatement", "t4cmarengine", "executequery", "executeupdate", "resultset")
	hasHTTP := containsAny(stack, "resttemplate", "webclient", "httpclient", "okhttp", "urlconnection", "mainclientexec", "bhttpconnection")
	hasNetwork := containsAny(stack, "socketinputstream.socketread", "niosocketimpl", "socketread", "socket.read", "netpoll", "epollwait")
	hasPool := containsAny(stack, "hikaripool.getconnection", "concurrentbag", "synchronousqueue")
	hasLock := containsAny(stack, "locksupport.park", "unsafe.park", "object.wait", "future.get", "monitor.enter", "mutex.lock")
	hasFile := containsAny(stack, "fileinputstream", "filechannel", "randomaccessfile", "bufferedreader.readline")
	hasGC := containsAny(stack, "g1youngcollector", "shenandoah", "zgc", "safepoint", "garbagecollect")

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

func looksLikeInternal(path []string) bool {
	stack := strings.ToLower(strings.Join(path, ";"))
	return containsAny(stack, "com.", "org.", "service", "controller", "processor", "writer", "reader", "tasklet", "job")
}
