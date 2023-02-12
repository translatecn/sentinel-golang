package api

import (
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/pkg/errors"
)

// TraceError 将提供的错误记录到给定的SentinelEntry
func TraceError(entry *base.SentinelEntry, err error) {
	defer func() {
		if e := recover(); e != nil {
			logging.Error(errors.Errorf("%+v", e), "Failed to api.TraceError()")
			return
		}
	}()
	if entry == nil || err == nil {
		return
	}

	entry.SetError(err)
}
