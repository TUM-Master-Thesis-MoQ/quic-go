package quic

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/quic-go/quic-go/internal/mocks"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/testutils"
	"github.com/quic-go/quic-go/internal/wire"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Stream", func() {
	const streamID protocol.StreamID = 1337

	var (
		str            *stream
		strWithTimeout io.ReadWriter // str wrapped with gbytes.Timeout{Reader,Writer}
		mockFC         *mocks.MockStreamFlowController
		mockSender     *MockStreamSender
	)

	BeforeEach(func() {
		mockSender = NewMockStreamSender(mockCtrl)
		mockFC = mocks.NewMockStreamFlowController(mockCtrl)
		str = newStream(streamID, mockSender, mockFC)

		timeout := testutils.ScaleDuration(250 * time.Millisecond)
		strWithTimeout = struct {
			io.Reader
			io.Writer
		}{
			gbytes.TimeoutReader(str, timeout),
			gbytes.TimeoutWriter(str, timeout),
		}
	})

	It("gets stream id", func() {
		Expect(str.StreamID()).To(Equal(protocol.StreamID(1337)))
	})

	Context("deadlines", func() {
		It("sets a write deadline, when SetDeadline is called", func() {
			str.SetDeadline(time.Now().Add(-time.Second))
			n, err := strWithTimeout.Write([]byte("foobar"))
			Expect(err).To(MatchError(errDeadline))
			Expect(n).To(BeZero())
		})

		It("sets a read deadline, when SetDeadline is called", func() {
			mockFC.EXPECT().UpdateHighestReceived(protocol.ByteCount(6), false).AnyTimes()
			f := &wire.StreamFrame{Data: []byte("foobar")}
			err := str.handleStreamFrame(f)
			Expect(err).ToNot(HaveOccurred())
			str.SetDeadline(time.Now().Add(-time.Second))
			b := make([]byte, 6)
			n, err := strWithTimeout.Read(b)
			Expect(err).To(MatchError(errDeadline))
			Expect(n).To(BeZero())
		})
	})

	Context("completing", func() {
		It("is not completed when only the receive side is completed", func() {
			// don't EXPECT a call to mockSender.onStreamCompleted()
			str.receiveStream.sender.onStreamCompleted(streamID)
		})

		It("is not completed when only the send side is completed", func() {
			// don't EXPECT a call to mockSender.onStreamCompleted()
			str.sendStream.sender.onStreamCompleted(streamID)
		})

		It("is completed when both sides are completed", func() {
			mockSender.EXPECT().onStreamCompleted(streamID)
			str.sendStream.sender.onStreamCompleted(streamID)
			str.receiveStream.sender.onStreamCompleted(streamID)
		})
	})
})

var _ = Describe("Deadline Error", func() {
	It("is a net.Error that wraps os.ErrDeadlineError", func() {
		err := deadlineError{}
		Expect(err.Timeout()).To(BeTrue())
		Expect(errors.Is(err, os.ErrDeadlineExceeded)).To(BeTrue())
		Expect(errors.Unwrap(err)).To(Equal(os.ErrDeadlineExceeded))
	})
})
