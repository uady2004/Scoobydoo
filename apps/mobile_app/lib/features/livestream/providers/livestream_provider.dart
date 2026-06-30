import 'dart:async';

import 'package:flutter/foundation.dart';

import '../models/livestream_model.dart';
import '../repositories/livestream_repository.dart';
import '../services/livestream_websocket_service.dart';

/// [LivestreamViewerProvider] manages viewer-side state for watching a live
/// stream — viewer count, chat messages, active poll, PK battle, gift
/// animations, and the stream lifecycle.
class LivestreamViewerProvider extends ChangeNotifier {
  LivestreamViewerProvider({
    required LivestreamRepository repo,
    required LivestreamWebSocketService wsService,
  })  : _repo = repo,
        _wsService = wsService;

  final LivestreamRepository _repo;
  final LivestreamWebSocketService _wsService;

  // --- Core state -------------------------------------------------------

  LiveStream? _stream;
  LiveStream? get stream => _stream;

  bool _loading = false;
  bool get loading => _loading;

  String? _error;
  String? get error => _error;

  int _viewerCount = 0;
  int get viewerCount => _viewerCount;

  // --- Chat state -------------------------------------------------------

  final List<LiveMessage> _messages = [];
  List<LiveMessage> get messages => List.unmodifiable(_messages);

  LiveMessage? _pinnedMessage;
  LiveMessage? get pinnedMessage => _pinnedMessage;

  // --- Poll state -------------------------------------------------------

  Poll? _activePoll;
  Poll? get activePoll => _activePoll;

  String? _votedOptionId; // prevents double-vote in UI
  String? get votedOptionId => _votedOptionId;

  // --- PK Battle state --------------------------------------------------

  PKBattle? _activeBattle;
  PKBattle? get activeBattle => _activeBattle;

  // --- Gift animation queue --------------------------------------------

  final List<GiftSentEvent> _giftQueue = [];
  List<GiftSentEvent> get giftQueue => List.unmodifiable(_giftQueue);

  // --- Gift catalog ----------------------------------------------------

  List<GiftType> _giftCatalog = [];
  List<GiftType> get giftCatalog => List.unmodifiable(_giftCatalog);

  // --- Subscriptions ---------------------------------------------------

  final List<StreamSubscription<dynamic>> _subs = [];

  // =========================================================================
  // Public API
  // =========================================================================

  /// Loads the stream, joins it server-side, connects the WebSocket, and
  /// subscribes to all real-time events.
  Future<void> joinStream(String streamId) async {
    _setLoading(true);
    try {
      _stream = await _repo.getStream(streamId);
      _viewerCount = _stream!.viewerCount;

      // Join server-side (increments viewer count in Redis).
      await _repo.joinStream(streamId);

      // Load initial chat history.
      final history = await _repo.getMessages(streamId);
      _messages
        ..clear()
        ..addAll(history.reversed);

      // Load active poll (if any).
      _activePoll = await _repo.getActivePoll(streamId);

      // Load active battle (if any).
      if (_stream!.pkBattleId != null) {
        _activeBattle = await _repo.getBattle(_stream!.pkBattleId!);
      }

      // Load gift catalog.
      _giftCatalog = await _repo.getGiftCatalog();

      // Connect WebSocket and wire up subscriptions.
      await _wsService.connect(streamId);
      _wireWebSocketSubscriptions();

      _setError(null);
    } catch (e) {
      _setError(e.toString());
    } finally {
      _setLoading(false);
    }
  }

  /// Gracefully leaves the stream.
  Future<void> leaveStream() async {
    if (_stream == null) return;
    await _repo.leaveStream(_stream!.id);
    await _disposeSubscriptions();
    await _wsService.disconnect();
    _stream = null;
    notifyListeners();
  }

  /// Sends a text chat message.
  Future<void> sendMessage(String content) async {
    if (_stream == null) return;
    try {
      final msg = await _repo.sendMessage(_stream!.id, content);
      // Optimistic insert handled by the WS echo; we ignore REST response here
      // unless WS is disconnected.
      if (!_wsService.isConnected) {
        _appendMessage(msg);
      }
    } catch (e) {
      _setError('Failed to send message: $e');
    }
  }

  /// Sends a gift.
  Future<void> sendGift(String giftTypeId, {int quantity = 1}) async {
    if (_stream == null) return;
    try {
      await _repo.sendGift(SendGiftRequest(
        streamId: _stream!.id,
        giftTypeId: giftTypeId,
        quantity: quantity,
      ));
      // The WS event will trigger the animation.
    } catch (e) {
      _setError('Failed to send gift: $e');
    }
  }

  /// Votes on a poll option.
  Future<void> votePoll(String pollId, String optionId) async {
    if (_votedOptionId != null) return; // already voted
    _votedOptionId = optionId;
    notifyListeners();
    try {
      final updated = await _repo.votePoll(pollId, optionId);
      _activePoll = updated;
      notifyListeners();
    } catch (e) {
      _votedOptionId = null;
      _setError('Failed to vote: $e');
    }
  }

  /// Dismisses the current gift animation (called by animation widget).
  void dismissGiftAnimation() {
    if (_giftQueue.isNotEmpty) {
      _giftQueue.removeAt(0);
      notifyListeners();
    }
  }

  // =========================================================================
  // Private helpers
  // =========================================================================

  void _wireWebSocketSubscriptions() {
    _subs.add(_wsService.viewerCounts.listen((count) {
      _viewerCount = count;
      notifyListeners();
    }));

    _subs.add(_wsService.chatMessages.listen((msg) {
      _appendMessage(msg);
      notifyListeners();
    }));

    _subs.add(_wsService.deletedMessageIds.listen((id) {
      _messages.removeWhere((m) => m.id == id);
      if (_pinnedMessage?.id == id) _pinnedMessage = null;
      notifyListeners();
    }));

    _subs.add(_wsService.pinnedMessageIds.listen((id) {
      _pinnedMessage = _messages.firstWhere(
        (m) => m.id == id,
        orElse: () => _pinnedMessage!,
      );
      notifyListeners();
    }));

    _subs.add(_wsService.giftAnimations.listen((gift) {
      _giftQueue.add(gift);
      notifyListeners();
    }));

    _subs.add(_wsService.pkScoreUpdates.listen((battle) {
      _activeBattle = battle;
      notifyListeners();
    }));

    _subs.add(_wsService.pkBattleEnd.listen((battle) {
      _activeBattle = battle;
      notifyListeners();
    }));

    _subs.add(_wsService.pollUpdates.listen((poll) {
      _activePoll = poll;
      notifyListeners();
    }));

    _subs.add(_wsService.streamEndedSignal.listen((_) {
      _stream = _stream?.copyWith(status: StreamStatus.ended);
      notifyListeners();
    }));
  }

  void _appendMessage(LiveMessage msg) {
    // Keep the chat window bounded to 200 messages.
    if (_messages.length >= 200) _messages.removeAt(0);
    _messages.add(msg);
  }

  void _setLoading(bool v) {
    _loading = v;
    notifyListeners();
  }

  void _setError(String? v) {
    _error = v;
    notifyListeners();
  }

  Future<void> _disposeSubscriptions() async {
    for (final s in _subs) {
      await s.cancel();
    }
    _subs.clear();
  }

  @override
  void dispose() {
    _disposeSubscriptions();
    _wsService.disconnect();
    super.dispose();
  }
}

// ---------------------------------------------------------------------------

/// [LivestreamHostProvider] extends the viewer state with host-specific
/// capabilities: starting/ending a stream, moderating chat, PK battles,
/// co-host management, and polls.
class LivestreamHostProvider extends ChangeNotifier {
  LivestreamHostProvider({
    required LivestreamRepository repo,
    required LivestreamWebSocketService wsService,
  })  : _repo = repo,
        _wsService = wsService;

  final LivestreamRepository _repo;
  final LivestreamWebSocketService _wsService;

  LiveStream? _stream;
  LiveStream? get stream => _stream;

  bool _loading = false;
  bool get loading => _loading;

  String? _error;
  String? get error => _error;

  bool _streaming = false;
  bool get streaming => _streaming;

  final List<StreamSubscription<dynamic>> _subs = [];

  // --- Stream setup fields (before going live) -------------------------
  String _title = '';
  String get title => _title;
  void setTitle(String v) {
    _title = v;
    notifyListeners();
  }

  String _description = '';
  String get description => _description;
  void setDescription(String v) {
    _description = v;
    notifyListeners();
  }

  bool _allowComments = true;
  bool get allowComments => _allowComments;
  void toggleAllowComments() {
    _allowComments = !_allowComments;
    notifyListeners();
  }

  // =========================================================================
  // Public API
  // =========================================================================

  /// Creates the stream on the backend and returns the RTMP ingest URL.
  Future<String?> startStream() async {
    if (_title.isEmpty) {
      _setError('Stream title is required');
      return null;
    }
    _setLoading(true);
    try {
      _stream = await _repo.startStream(StartStreamRequest(
        title: _title,
        description: _description,
        allowComments: _allowComments,
      ));
      _streaming = true;
      // Connect WS to receive viewer events.
      await _wsService.connect(_stream!.id);
      _wireSubscriptions();
      _setError(null);
      return _stream!.rtmpKey;
    } catch (e) {
      _setError(e.toString());
      return null;
    } finally {
      _setLoading(false);
    }
  }

  /// Ends the stream and disconnects.
  Future<void> endStream() async {
    if (_stream == null) return;
    _setLoading(true);
    try {
      await _repo.endStream(_stream!.id);
      _streaming = false;
      _stream = _stream!.copyWith(status: StreamStatus.ended);
      await _disposeSubscriptions();
      await _wsService.disconnect();
      notifyListeners();
    } catch (e) {
      _setError(e.toString());
    } finally {
      _setLoading(false);
    }
  }

  /// Deletes a chat message as the host.
  Future<void> deleteMessage(String messageId) async {
    if (_stream == null) return;
    await _repo.deleteMessage(_stream!.id, messageId);
  }

  /// Pins a chat message.
  Future<void> pinMessage(String messageId) async {
    if (_stream == null) return;
    await _repo.pinMessage(_stream!.id, messageId);
  }

  /// Invites another live streamer to a PK battle.
  Future<PKBattle?> inviteToBattle(String targetUserId, {int durationSecs = 60}) async {
    if (_stream == null) return null;
    try {
      final battle = await _repo.inviteToBattle(
        _stream!.id,
        targetUserId,
        durationSecs: durationSecs,
      );
      return battle;
    } catch (e) {
      _setError(e.toString());
      return null;
    }
  }

  /// Creates an audience poll.
  Future<Poll?> createPoll({
    required String question,
    required List<String> options,
    int durationSecs = 60,
  }) async {
    if (_stream == null) return null;
    try {
      return await _repo.createPoll(CreatePollRequest(
        streamId: _stream!.id,
        question: question,
        options: options,
        durationSecs: durationSecs,
      ));
    } catch (e) {
      _setError(e.toString());
      return null;
    }
  }

  // =========================================================================
  // Private helpers
  // =========================================================================

  void _wireSubscriptions() {
    _subs.add(_wsService.streamEndedSignal.listen((_) {
      _streaming = false;
      notifyListeners();
    }));
  }

  void _setLoading(bool v) {
    _loading = v;
    notifyListeners();
  }

  void _setError(String? v) {
    _error = v;
    notifyListeners();
  }

  Future<void> _disposeSubscriptions() async {
    for (final s in _subs) {
      await s.cancel();
    }
    _subs.clear();
  }

  @override
  void dispose() {
    _disposeSubscriptions();
    _wsService.disconnect();
    super.dispose();
  }
}
