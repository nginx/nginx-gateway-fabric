//! gRPC length-prefixed framing.
//!
//! Each gRPC message is preceded by a 5-byte header: 1 byte compression flag
//! (must be 0 — we do not support compression) and 4 bytes big-endian payload
//! length. These helpers encode and decode that framing without any I/O.

use bytes::{Buf, Bytes, BytesMut};
use prost::Message;

use crate::error::ModuleError;

/// Size of the gRPC frame header (compression flag + u32 length).
const HEADER_LEN: usize = 5;

/// Check both the wire length and the final `Vec<u8>` allocation length.
pub(crate) fn checked_frame_len(payload_len: usize) -> Result<(u32, usize), ModuleError> {
    let wire_len = u32::try_from(payload_len)
        .map_err(|_| ModuleError::GrpcFrame("payload exceeds gRPC u32 length".into()))?;
    let frame_len = payload_len
        .checked_add(HEADER_LEN)
        .filter(|&len| len <= isize::MAX as usize)
        .ok_or_else(|| ModuleError::GrpcFrame("frame allocation length overflow".into()))?;
    Ok((wire_len, frame_len))
}

/// Encode directly into the final uncompressed gRPC frame, without a second
/// payload-sized buffer. Reserve before writing so allocation failure is fallible.
pub(crate) fn encode_message(message: &impl Message) -> Result<Vec<u8>, ModuleError> {
    let (wire_len, frame_len) = checked_frame_len(message.encoded_len())?;
    let mut frame = Vec::new();
    frame
        .try_reserve_exact(frame_len)
        .map_err(|e| ModuleError::GrpcFrame(format!("cannot allocate frame: {e}")))?;
    frame.push(0);
    frame.extend_from_slice(&wire_len.to_be_bytes());
    message
        .encode(&mut frame)
        .map_err(|e| ModuleError::GrpcFrame(format!("cannot encode protobuf message: {e}")))?;
    Ok(frame)
}

/// Wrap `payload` in a gRPC length-prefixed frame (uncompressed).
#[cfg(test)]
pub(crate) fn encode(payload: &[u8]) -> Vec<u8> {
    let (len, frame_len) = checked_frame_len(payload.len()).unwrap();
    let mut frame = Vec::with_capacity(frame_len);
    frame.push(0); // no compression
    frame.extend_from_slice(&len.to_be_bytes());
    frame.extend_from_slice(payload);
    frame
}

/// A decoded message payload and the remaining unconsumed bytes.
pub(crate) type Split<'a> = (&'a [u8], &'a [u8]);

/// Decode one gRPC length-prefixed message from the front of `buf`.
///
/// Returns `Ok(Some((payload, rest)))` when a complete frame is available,
/// `Ok(None)` when more bytes are needed, and `Err` when the compression flag
/// is set.
pub(crate) fn decode_partial(buf: &[u8]) -> Result<Option<Split<'_>>, ModuleError> {
    if buf.is_empty() {
        return Ok(None);
    }
    if buf[0] != 0 {
        return Err(ModuleError::GrpcFrame(
            "compressed frames are not supported".into(),
        ));
    }
    if buf.len() < HEADER_LEN {
        return Ok(None);
    }

    let len = u32::from_be_bytes([buf[1], buf[2], buf[3], buf[4]]) as usize;
    let Some(total) = HEADER_LEN.checked_add(len) else {
        return Err(ModuleError::GrpcFrame("frame length overflow".into()));
    };
    if buf.len() < total {
        return Ok(None);
    }

    Ok(Some((&buf[HEADER_LEN..total], &buf[total..])))
}

/// Accumulates DATA chunks from a stream and yields complete gRPC message
/// payloads without re-scanning already-consumed bytes.
pub(crate) struct FrameReader {
    buf: BytesMut,
    limit: usize,
}

impl FrameReader {
    /// Create a reader that rejects more than `limit` buffered bytes.
    pub(crate) fn new(limit: usize) -> Self {
        Self {
            buf: BytesMut::new(),
            limit,
        }
    }

    /// Append a chunk of stream data.
    pub(crate) fn push(&mut self, chunk: &[u8]) -> Result<(), ModuleError> {
        if chunk.len() > self.limit.saturating_sub(self.buf.len()) {
            return Err(ModuleError::GrpcFrame(format!(
                "message exceeds {} byte limit",
                self.limit
            )));
        }
        self.buf.extend_from_slice(chunk);
        Ok(())
    }

    /// Take the next complete message payload, if one is buffered.
    ///
    /// Call this before appending more data with `push_partial`.
    pub(crate) fn next_message(&mut self) -> Result<Option<Bytes>, ModuleError> {
        if self.buf.len() >= HEADER_LEN {
            let len =
                u32::from_be_bytes([self.buf[1], self.buf[2], self.buf[3], self.buf[4]]) as usize;
            if len > self.limit.saturating_sub(HEADER_LEN) {
                return Err(ModuleError::GrpcFrame(format!(
                    "message exceeds {} byte limit",
                    self.limit
                )));
            }
        }
        let Some((payload, _)) = decode_partial(&self.buf)? else {
            return Ok(None);
        };
        let mut frame = self.buf.split_to(HEADER_LEN + payload.len());
        frame.advance(HEADER_LEN);
        Ok(Some(frame.freeze()))
    }

    /// Copy only what fits, leaving the rest of this DATA chunk for the next
    /// message. Drain complete messages before calling this again.
    pub(crate) fn push_partial(&mut self, chunk: &mut Bytes) -> Result<(), ModuleError> {
        let len = chunk.len().min(self.limit.saturating_sub(self.buf.len()));
        if len == 0 && !chunk.is_empty() {
            return Err(ModuleError::GrpcFrame(
                "frame buffer cannot make progress".into(),
            ));
        }
        self.push(&chunk[..len])?;
        chunk.advance(len);
        Ok(())
    }

    /// Whether any unconsumed bytes remain (i.e. a partial frame).
    pub(crate) fn has_partial(&self) -> bool {
        !self.buf.is_empty()
    }
}

/// Decode a single, complete gRPC message from `buf`.
///
/// Like [`decode_partial`], but treats an incomplete frame as an error.
#[cfg(test)]
pub(crate) fn decode(buf: &[u8]) -> Result<(&[u8], &[u8]), ModuleError> {
    decode_partial(buf)?.ok_or_else(|| {
        ModuleError::GrpcFrame(format!("incomplete frame ({} bytes available)", buf.len()))
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn frame_length_boundaries_without_allocating() {
        let max_payload = (u32::MAX as usize).min(isize::MAX as usize - HEADER_LEN);
        assert_eq!(checked_frame_len(0).unwrap(), (0, HEADER_LEN));
        assert_eq!(
            checked_frame_len(max_payload).unwrap(),
            (
                u32::try_from(max_payload).unwrap(),
                max_payload + HEADER_LEN
            ),
        );
        for len in [max_payload + 1, usize::MAX - HEADER_LEN, usize::MAX] {
            assert!(matches!(
                checked_frame_len(len),
                Err(ModuleError::GrpcFrame(_))
            ));
        }
        if let Some(too_large) = (u32::MAX as usize).checked_add(1) {
            assert!(checked_frame_len(too_large).is_err());
        }
    }

    #[test]
    fn direct_encoding_matches_payload_framing() {
        use envoy_types::pb::envoy::service::ext_proc::v3::HttpBody;

        for body in [vec![], b"hello grpc".to_vec(), vec![0, 0xff, 0x80]] {
            for end_of_stream in [false, true] {
                let message = HttpBody {
                    body: body.clone(),
                    end_of_stream,
                    ..Default::default()
                };
                let frame = encode_message(&message).unwrap();
                assert_eq!(frame, encode(&message.encode_to_vec()));
                assert_eq!(frame.len(), HEADER_LEN + message.encoded_len());
                assert_eq!(
                    HttpBody::decode(decode(&frame).unwrap().0).unwrap(),
                    message
                );
            }
        }
    }

    #[test]
    fn round_trip() {
        let payload = b"hello grpc";
        let frame = encode(payload);
        let (decoded, rest) = decode(&frame).unwrap();
        assert_eq!(decoded, payload);
        assert!(rest.is_empty());
    }

    #[test]
    fn round_trip_empty() {
        let frame = encode(b"");
        let (decoded, rest) = decode(&frame).unwrap();
        assert!(decoded.is_empty());
        assert!(rest.is_empty());
    }

    #[test]
    fn decode_with_trailing_bytes() {
        let mut frame = encode(b"abc");
        frame.extend_from_slice(b"trailing");
        let (decoded, rest) = decode(&frame).unwrap();
        assert_eq!(decoded, b"abc");
        assert_eq!(rest, b"trailing");
    }

    #[test]
    fn decode_multiple_frames() {
        let mut buf = encode(b"first");
        buf.extend_from_slice(&encode(b"second"));
        let (first, rest) = decode(&buf).unwrap();
        assert_eq!(first, b"first");
        let (second, rest) = decode(rest).unwrap();
        assert_eq!(second, b"second");
        assert!(rest.is_empty());
    }

    #[test]
    fn decode_too_short() {
        assert!(decode(b"").is_err());
        assert!(decode(b"\x00\x00").is_err());
    }

    #[test]
    fn decode_compressed_rejected() {
        let mut frame = encode(b"x");
        frame[0] = 1; // set compression flag
        assert!(decode(&frame).is_err());
    }

    #[test]
    fn decode_truncated_payload() {
        let frame = encode(b"hello");
        // Chop off the last byte so the payload is incomplete.
        assert!(decode(&frame[..frame.len() - 1]).is_err());
    }

    #[test]
    fn decode_partial_incomplete_is_none() {
        assert!(decode_partial(b"").unwrap().is_none());
        assert!(decode_partial(b"\x00\x00").unwrap().is_none());
        let frame = encode(b"hello");
        assert!(decode_partial(&frame[..frame.len() - 1]).unwrap().is_none());
    }

    #[test]
    fn decode_partial_compressed_is_error() {
        assert!(decode_partial(b"\x01").is_err());
    }

    #[test]
    fn frame_reader_split_chunks() {
        let mut buf = encode(b"first");
        buf.extend_from_slice(&encode(b"second"));
        let mut r = FrameReader::new(1024);
        r.push(&buf[..3]).unwrap();
        assert!(r.next_message().unwrap().is_none());
        r.push(&buf[3..12]).unwrap();
        assert_eq!(&r.next_message().unwrap().unwrap()[..], b"first");
        assert!(r.next_message().unwrap().is_none());
        assert!(r.has_partial());
        r.push(&buf[12..]).unwrap();
        assert_eq!(&r.next_message().unwrap().unwrap()[..], b"second");
        assert!(!r.has_partial());
    }

    #[test]
    fn frame_reader_coalesced_frames_are_independent_of_chunk_boundaries() {
        let frame = encode(b"abc");
        let data = frame.repeat(3);
        for split in 0..=data.len() {
            let mut reader = FrameReader::new(frame.len());
            let mut messages = Vec::new();
            for part in [&data[..split], &data[split..]] {
                let mut chunk = Bytes::copy_from_slice(part);
                loop {
                    while let Some(message) = reader.next_message().unwrap() {
                        messages.push(message);
                    }
                    if chunk.is_empty() {
                        break;
                    }
                    reader.push_partial(&mut chunk).unwrap();
                    assert!(reader.buf.len() <= frame.len());
                }
            }
            assert_eq!(
                messages,
                vec![Bytes::from_static(b"abc"); 3],
                "split {split}"
            );
            assert!(!reader.has_partial());
        }
    }

    #[test]
    fn frame_reader_limit() {
        let mut r = FrameReader::new(4);
        assert!(r.push(b"12345").is_err());
    }

    #[test]
    fn frame_reader_rejects_oversized_advertised_length_before_payload() {
        let mut r = FrameReader::new(16);
        r.push(&[0, 0xff, 0xff]).unwrap();
        assert!(r.next_message().unwrap().is_none());
        r.push(&[0xff, 0xff]).unwrap();
        assert!(matches!(r.next_message(), Err(ModuleError::GrpcFrame(_))));
    }

    #[test]
    fn frame_reader_accepts_exact_limit_and_reuses_budget() {
        let frame = encode(b"abc");
        let mut r = FrameReader::new(frame.len());
        for _ in 0..3 {
            r.push(&frame).unwrap();
            assert_eq!(&r.next_message().unwrap().unwrap()[..], b"abc");
            assert!(!r.has_partial());
        }
    }

    #[test]
    fn frame_reader_rejected_chunk_does_not_change_buffer() {
        let frame = encode(b"abc");
        let mut r = FrameReader::new(frame.len());
        r.push(&frame[..3]).unwrap();
        assert!(r.push(&frame).is_err());
        r.push(&frame[3..]).unwrap();
        assert_eq!(&r.next_message().unwrap().unwrap()[..], b"abc");
    }
}
