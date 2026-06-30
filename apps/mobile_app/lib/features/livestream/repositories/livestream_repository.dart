import 'package:dio/dio.dart';

import '../models/livestream_model.dart';

/// [LivestreamRepository] talks to the livestream microservice REST API.
/// WebSocket events are handled separately in [LivestreamWebSocketService].
class LivestreamRepository {
  LivestreamRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;
  static const _base = '/v1/livestreams';

  // ---- Stream lifecycle -----------------------------------------------

  /// Starts a new stream and returns the created [LiveStream] (includes RTMP key).
  Future<LiveStream> startStream(StartStreamRequest req) async {
    final res = await _dio.post(_base, data: req.toJson());
    return LiveStream.fromJson(res.data as Map<String, dynamic>);
  }

  /// Ends a live stream.
  Future<void> endStream(String streamId) async {
    await _dio.patch('$_base/$streamId/end');
  }

  /// Fetches a single stream by ID.
  Future<LiveStream> getStream(String streamId) async {
    final res = await _dio.get('$_base/$streamId');
    return LiveStream.fromJson(res.data as Map<String, dynamic>);
  }

  /// Returns currently live streams, paginated.
  Future<List<LiveStream>> getActiveStreams({int limit = 20, int offset = 0}) async {
    final res = await _dio.get(
      '$_base/active',
      queryParameters: {'limit': limit, 'offset': offset},
    );
    final list = res.data as List<dynamic>;
    return list
        .map((e) => LiveStream.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Joins a stream as a viewer — returns the join payload (includes WS token).
  Future<Map<String, dynamic>> joinStream(String streamId) async {
    final res = await _dio.post('$_base/$streamId/join');
    return res.data as Map<String, dynamic>;
  }

  /// Signals the server that the current user left the stream.
  Future<void> leaveStream(String streamId) async {
    await _dio.post('$_base/$streamId/leave');
  }

  // ---- Chat -----------------------------------------------------------

  /// Fetches the last [limit] chat messages for a stream.
  Future<List<LiveMessage>> getMessages(String streamId, {int limit = 50}) async {
    final res = await _dio.get(
      '$_base/$streamId/messages',
      queryParameters: {'limit': limit},
    );
    final list = res.data as List<dynamic>;
    return list
        .map((e) => LiveMessage.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Sends a chat message. The server fans it out over WebSocket.
  Future<LiveMessage> sendMessage(String streamId, String content, {String type = 'text'}) async {
    final res = await _dio.post(
      '$_base/$streamId/messages',
      data: {'content': content, 'type': type},
    );
    return LiveMessage.fromJson(res.data as Map<String, dynamic>);
  }

  /// Pins a message (host/moderator only).
  Future<void> pinMessage(String streamId, String messageId) async {
    await _dio.patch('$_base/$streamId/messages/$messageId/pin');
  }

  /// Soft-deletes a message (host/moderator only).
  Future<void> deleteMessage(String streamId, String messageId) async {
    await _dio.delete('$_base/$streamId/messages/$messageId');
  }

  // ---- Gifts ----------------------------------------------------------

  /// Returns all available gift types from the catalog.
  Future<List<GiftType>> getGiftCatalog() async {
    final res = await _dio.get('/v1/gifts/catalog');
    final list = res.data as List<dynamic>;
    return list
        .map((e) => GiftType.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Sends a gift. Coin deduction and creator credit happen server-side.
  Future<void> sendGift(SendGiftRequest req) async {
    await _dio.post('/v1/gifts', data: req.toJson());
  }

  // ---- PK Battle ------------------------------------------------------

  /// Invites another streamer to a PK battle.
  Future<PKBattle> inviteToBattle(String streamId, String targetUserId, {int durationSecs = 60}) async {
    final res = await _dio.post(
      '$_base/$streamId/battle/invite',
      data: {'target_user_id': targetUserId, 'duration_secs': durationSecs},
    );
    return PKBattle.fromJson(res.data as Map<String, dynamic>);
  }

  /// Accepts a PK battle invite.
  Future<PKBattle> acceptBattle(String battleId) async {
    final res = await _dio.patch('/v1/battles/$battleId/accept');
    return PKBattle.fromJson(res.data as Map<String, dynamic>);
  }

  /// Declines a PK battle invite.
  Future<void> declineBattle(String battleId) async {
    await _dio.patch('/v1/battles/$battleId/decline');
  }

  /// Fetches current battle state (for reconnect).
  Future<PKBattle> getBattle(String battleId) async {
    final res = await _dio.get('/v1/battles/$battleId');
    return PKBattle.fromJson(res.data as Map<String, dynamic>);
  }

  // ---- Polls ----------------------------------------------------------

  /// Creates an in-stream poll.
  Future<Poll> createPoll(CreatePollRequest req) async {
    final res = await _dio.post('/v1/polls', data: req.toJson());
    return Poll.fromJson(res.data as Map<String, dynamic>);
  }

  /// Votes on a poll option.
  Future<Poll> votePoll(String pollId, String optionId) async {
    final res = await _dio.post('/v1/polls/$pollId/vote', data: {'option_id': optionId});
    return Poll.fromJson(res.data as Map<String, dynamic>);
  }

  /// Manually closes a poll (creator only).
  Future<void> closePoll(String pollId) async {
    await _dio.patch('/v1/polls/$pollId/close');
  }

  /// Fetches the active poll for a stream.
  Future<Poll?> getActivePoll(String streamId) async {
    try {
      final res = await _dio.get('$_base/$streamId/poll');
      return Poll.fromJson(res.data as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) return null;
      rethrow;
    }
  }

  // ---- HLS / Playback --------------------------------------------------

  /// Returns the HLS master playlist URL for a stream.
  String hlsMasterPlaylistUrl(String streamId) {
    final base = _dio.options.baseUrl.replaceAll('/api', '');
    return '$base/hls/$streamId/master.m3u8';
  }
}
