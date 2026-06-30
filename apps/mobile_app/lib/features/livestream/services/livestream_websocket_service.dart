import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import '../models/livestream_model.dart';

/// Event types that arrive over the WebSocket.
enum WsEventType {
  viewerJoin,
  viewerLeave,
  viewerCount,
  chatMessage,
  chatDelete,
  chatPin,
  giftSent,
  giftAnimation,
  streamEnd,
  pkInvite,
  pkStart,
  pkScore,
  pkEnd,
  pollCreate,
  pollVote,
  pollClose,
  coHostInvite,
  coHostAccept,
  coHostRemove,
  unknown,
}

class WsEvent {
  final WsEventType type;
  final String streamId;
  final DateTime timestamp;
  final Map<String, dynamic> payload;

  const WsEvent({
    required this.type,
    required this.streamId,
    required this.timestamp,
    required this.payload,
  });

  factory WsEvent.fromJson(Map<String, dynamic> json) {
    return WsEvent(
      type: _parseType(json['type'] as String? ?? ''),
      streamId: json['stream_id'] as String? ?? '',
      timestamp: json['timestamp'] != null
          ? DateTime.parse(json['timestamp'] as String)
          : DateTime.now(),
      payload: json['payload'] as Map<String, dynamic>? ?? {},
    );
  }

  static WsEventType _parseType(String t) {
    switch (t) {
      case 'viewer.join':
        return WsEventType.viewerJoin;
      case 'viewer.leave':
        return WsEventType.viewerLeave;
      case 'viewer.count':
        return WsEventType.viewerCount;
      case 'chat.message':
        return WsEventType.chatMessage;
      case 'chat.delete':
        return WsEventType.chatDelete;
      case 'chat.pin':
        return WsEventType.chatPin;
      case 'gift.sent':
        return WsEventType.giftSent;
      case 'gift.animation':
        return WsEventType.giftAnimation;
      case 'stream.end':
        return WsEventType.streamEnd;
      case 'pk.invite':
        return WsEventType.pkInvite;
      case 'pk.start':
        return WsEventType.pkStart;
      case 'pk.score':
        return WsEventType.pkScore;
      case 'pk.end':
        return WsEventType.pkEnd;
      case 'poll.create':
        return WsEventType.pollCreate;
      case 'poll.vote':
        return WsEventType.pollVote;
      case 'poll.close':
        return WsEventType.pollClose;
      case 'cohost.invite':
        return WsEventType.coHostInvite;
      case 'cohost.accept':
        return WsEventType.coHostAccept;
      case 'cohost.remove':
        return WsEventType.coHostRemove;
      default:
        return WsEventType.unknown;
    }
  }
}

/// [LivestreamWebSocketService] manages the persistent WebSocket connection
/// for a single stream room. It exposes typed [Stream]s for each event
/// category so consumers can listen selectively.
class LivestreamWebSocketService {
  LivestreamWebSocketService({required String wsBaseUrl, required String authToken})
      : _wsBaseUrl = wsBaseUrl,
        _authToken = authToken;

  final String _wsBaseUrl;
  final String _authToken;

  WebSocketChannel? _channel;
  StreamController<WsEvent>? _eventController;
  StreamSubscription<dynamic>? _sub;

  bool _connected = false;
  bool get isConnected => _connected;

  // Individual typed streams derived from the master event stream.
  Stream<WsEvent>? _events;
  Stream<WsEvent> get events => _events ?? const Stream.empty();

  Stream<LiveMessage> get chatMessages => events
      .where((e) => e.type == WsEventType.chatMessage)
      .map((e) => LiveMessage.fromJson(e.payload));

  Stream<String> get deletedMessageIds => events
      .where((e) => e.type == WsEventType.chatDelete)
      .map((e) => e.payload['message_id'] as String);

  Stream<String> get pinnedMessageIds => events
      .where((e) => e.type == WsEventType.chatPin)
      .map((e) => e.payload['message_id'] as String);

  Stream<int> get viewerCounts => events
      .where((e) => e.type == WsEventType.viewerCount)
      .map((e) => e.payload['current'] as int? ?? 0);

  Stream<GiftSentEvent> get giftAnimations => events
      .where((e) => e.type == WsEventType.giftAnimation)
      .map((e) => GiftSentEvent.fromJson(e.payload));

  Stream<PKBattle> get pkScoreUpdates => events
      .where((e) => e.type == WsEventType.pkScore)
      .map((e) => PKBattle.fromJson(e.payload));

  Stream<PKBattle> get pkBattleEnd => events
      .where((e) => e.type == WsEventType.pkEnd)
      .map((e) => PKBattle.fromJson(e.payload));

  Stream<Poll> get pollUpdates => events
      .where((e) =>
          e.type == WsEventType.pollCreate ||
          e.type == WsEventType.pollVote ||
          e.type == WsEventType.pollClose)
      .map((e) => Poll.fromJson(e.payload));

  Stream<void> get streamEndedSignal => events
      .where((e) => e.type == WsEventType.streamEnd)
      .map((_) {});

  /// Connects to the WebSocket room for [streamId].
  Future<void> connect(String streamId) async {
    if (_connected) await disconnect();

    final uri = Uri.parse('$_wsBaseUrl/ws/$streamId?token=$_authToken');
    _channel = WebSocketChannel.connect(uri);
    _eventController = StreamController<WsEvent>.broadcast();
    _events = _eventController!.stream;

    _sub = _channel!.stream.listen(
      (raw) {
        try {
          final json = jsonDecode(raw as String) as Map<String, dynamic>;
          _eventController!.add(WsEvent.fromJson(json));
        } catch (_) {
          // Ignore malformed frames.
        }
      },
      onError: (Object err) {
        _eventController!.addError(err);
      },
      onDone: () {
        _connected = false;
        _eventController!.close();
      },
    );
    _connected = true;
  }

  /// Sends a raw JSON payload over the WebSocket (e.g. for chat).
  void send(Map<String, dynamic> payload) {
    if (!_connected || _channel == null) return;
    _channel!.sink.add(jsonEncode(payload));
  }

  /// Disconnects and cleans up resources.
  Future<void> disconnect() async {
    _connected = false;
    await _sub?.cancel();
    await _channel?.sink.close();
    await _eventController?.close();
    _channel = null;
    _eventController = null;
    _sub = null;
  }
}
