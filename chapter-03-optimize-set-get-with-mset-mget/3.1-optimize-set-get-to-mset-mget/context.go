package main

// IContext is the context for service
type IContext interface {
	Log(message string)
	Param(name string) string
	QueryParam(name string) string
	ReadInput() string
	Response(responseCode int, responseData interface{})
	ResponseS(responseCode int, responseData string)

	Cacher(cfg ICacherConfig) ICacher
	Persister(cfg IPersisterConfig) IPersister
	MemCacher() IMemCacher
}
