package stream

import (
	"github.com/getgauge/gauge/gauge_messages"
	"github.com/getgauge/gauge/logger"
	"github.com/golang/protobuf/proto"
)

// ExecutionStream represents the server for streaming execution
type ExecutionStream struct{}

func (s *ExecutionStream) Execute(request *gauge_messages.ExecutionRequest, stream gauge_messages.Execution_ExecuteServer) error {
	logger.Info("got exe request")

	// specs := request.GetSpecs()
	for i := 0; i < 10; i++ {
		stream.Send(&gauge_messages.ExecutionResponse{Type: gauge_messages.ExecutionResponse_ScenarioResult.Enum(), ID: proto.String("random id")})
	}

	return nil
}
