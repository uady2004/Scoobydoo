import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/usecases/get_conversations_usecase.dart';
import 'package:tiktok_clone/features/messaging/domain/usecases/get_messages_usecase.dart';
import 'package:tiktok_clone/features/messaging/domain/usecases/send_message_usecase.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/auth/presentation/providers/auth_provider.dart';
import 'package:tiktok_clone/features/messaging/data/datasources/message_remote_datasource.dart';
import 'package:tiktok_clone/features/messaging/data/repositories/message_repository_impl.dart';
import 'package:tiktok_clone/features/messaging/domain/repositories/message_repository.dart';

// ---------------------------------------------------------------------------
// WebSocketService
// ---------------------------------------------------------------------------

class WebSocketService {
  WebSocketChannel? _channel;
  final _controller = StreamController<Map<String, dynamic>>.broadcast();

  Stream<Map<String, dynamic>> get messageStream => _controller.stream;

  bool get isConnected => _channel != null;

  void connect(String token, {String wsUrl = 'wss://api.example.com/ws'}) {
    _channel?.sink.close();
    _channel = WebSocketChannel.connect(
      Uri.parse('$wsUrl?token=$token'),
    );
    _channel!.stream.listen(
      (raw) {
        try {
          final decoded = jsonDecode(raw as String) as Map<String, dynamic>;
          _controller.add(decoded);
        } catch (_) {
          // ignore malformed frames
        }
      },
      onError: (_) => _scheduleReconnect(token, wsUrl),
      onDone: () => _scheduleReconnect(token, wsUrl),
    );
  }

  void disconnect() {
    _channel?.sink.close();
    _channel = null;
  }

  void sendWsMessage(Map<String, dynamic> json) {
    _channel?.sink.add(jsonEncode(json));
  }

  void _scheduleReconnect(String token, String wsUrl) {
    Future.delayed(const Duration(seconds: 3), () {
      if (!_controller.isClosed) connect(token, wsUrl: wsUrl);
    });
  }

  void dispose() {
    disconnect();
    _controller.close();
  }
}

// ---------------------------------------------------------------------------
// Providers — infrastructure
// ---------------------------------------------------------------------------

final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  final service = WebSocketService();
  ref.onDispose(service.dispose);
  return service;
});

final _messageDatasourceProvider = Provider<MessageRemoteDataSource>((ref) =>
    MessageRemoteDataSourceImpl(ApiClient.instance.dio));

final _messageRepoProvider = Provider<MessageRepository>((ref) =>
    MessageRepositoryImpl(ref.watch(_messageDatasourceProvider)));

final getConversationsUseCaseProvider =
    Provider<GetConversationsUseCase>((ref) =>
        GetConversationsUseCase(ref.watch(_messageRepoProvider)));

final getMessagesUseCaseProvider = Provider<GetMessagesUseCase>((ref) =>
    GetMessagesUseCase(ref.watch(_messageRepoProvider)));

final sendMessageUseCaseProvider = Provider<SendMessageUseCase>((ref) =>
    SendMessageUseCase(ref.watch(_messageRepoProvider)));

final currentUserIdProvider = Provider<String>((ref) {
  final authAsync = ref.watch(authProvider);
  final authState = authAsync.valueOrNull;
  if (authState is AuthAuthenticated) return authState.user.id;
  return '';
});

// ---------------------------------------------------------------------------
// InboxNotifier
// ---------------------------------------------------------------------------

class InboxNotifier extends AsyncNotifier<List<ConversationEntity>> {
  StreamSubscription<Map<String, dynamic>>? _wsSub;
  String? _nextCursor;
  bool _isLoadingMore = false;

  @override
  Future<List<ConversationEntity>> build() async {
    ref.onDispose(() => _wsSub?.cancel());
    final result = await ref
        .read(getConversationsUseCaseProvider)
        .call(cursor: null);
    return result.fold(
      (err) => throw Exception(err),
      (tuple) {
        _nextCursor = tuple.$2;
        _listenToWebSocket();
        return tuple.$1;
      },
    );
  }

  void _listenToWebSocket() {
    _wsSub?.cancel();
    _wsSub = ref
        .read(webSocketServiceProvider)
        .messageStream
        .where((e) => e['event'] == 'new_message' || e['event'] == 'read')
        .listen(_handleWsEvent);
  }

  void _handleWsEvent(Map<String, dynamic> event) {
    final current = state.valueOrNull;
    if (current == null) return;

    if (event['event'] == 'new_message') {
      final convId = event['conversation_id'] as String?;
      if (convId == null) return;
      final idx = current.indexWhere((c) => c.id == convId);
      if (idx == -1) return;
      final conv = current[idx];
      // Move conversation to top with updated unread count.
      final updated = current.toList()..removeAt(idx);
      updated.insert(
        0,
        conv.copyWith(
          unreadCount: conv.unreadCount + 1,
          lastMessageAt: DateTime.now(),
        ),
      );
      state = AsyncData(updated);
    } else if (event['event'] == 'read') {
      final convId = event['conversation_id'] as String?;
      if (convId == null) return;
      final updated = current.map((c) {
        if (c.id == convId) return c.copyWith(unreadCount: 0);
        return c;
      }).toList();
      state = AsyncData(updated);
    }
  }

  Future<void> loadMore() async {
    if (_isLoadingMore || _nextCursor == null) return;
    _isLoadingMore = true;
    final current = state.valueOrNull ?? [];
    final result = await ref
        .read(getConversationsUseCaseProvider)
        .call(cursor: _nextCursor);
    result.fold(
      (_) {},
      (tuple) {
        _nextCursor = tuple.$2;
        state = AsyncData([...current, ...tuple.$1]);
      },
    );
    _isLoadingMore = false;
  }

  Future<void> refresh() async {
    _nextCursor = null;
    state = const AsyncLoading();
    state = await AsyncValue.guard(() async {
      final result = await ref
          .read(getConversationsUseCaseProvider)
          .call(cursor: null);
      return result.fold(
        (err) => throw Exception(err),
        (tuple) {
          _nextCursor = tuple.$2;
          return tuple.$1;
        },
      );
    });
  }
}

final inboxProvider =
    AsyncNotifierProvider<InboxNotifier, List<ConversationEntity>>(
  InboxNotifier.new,
);

// ---------------------------------------------------------------------------
// ChatNotifier  (family by conversationId)
// ---------------------------------------------------------------------------

class ChatNotifier extends FamilyAsyncNotifier<List<MessageEntity>, String> {
  StreamSubscription<Map<String, dynamic>>? _wsSub;
  String? _nextCursor;
  bool _isLoadingMore = false;
  late String _convId;

  @override
  Future<List<MessageEntity>> build(String arg) async {
    _convId = arg;
    ref.onDispose(() => _wsSub?.cancel());
    final result = await ref
        .read(getMessagesUseCaseProvider)
        .call(_convId, cursor: null);
    return result.fold(
      (err) => throw Exception(err),
      (tuple) {
        _nextCursor = tuple.$2;
        _listenToWebSocket();
        return tuple.$1;
      },
    );
  }

  void _listenToWebSocket() {
    _wsSub?.cancel();
    _wsSub = ref
        .read(webSocketServiceProvider)
        .messageStream
        .where((e) =>
            e['event'] == 'new_message' &&
            e['conversation_id'] == _convId)
        .listen(_handleNewMessage);
  }

  void _handleNewMessage(Map<String, dynamic> event) {
    final current = state.valueOrNull;
    if (current == null) return;
    try {
      final msgData = event['message'] as Map<String, dynamic>;
      // Replace optimistic message if present, else prepend.
      final idx = current.indexWhere((m) =>
          m.id.startsWith('temp_') &&
          m.content == msgData['content']);
      final updated = current.toList();
      if (idx != -1) {
        // Replace optimistic placeholder with confirmed message.
        // We import MessageModel here via a factory on entity level.
        updated[idx] = _messageFromWs(msgData);
      } else {
        updated.insert(0, _messageFromWs(msgData));
      }
      state = AsyncData(updated);
    } catch (_) {}
  }

  MessageEntity _messageFromWs(Map<String, dynamic> data) {
    return MessageEntity(
      id: data['id'] as String? ?? '',
      conversationId: _convId,
      senderId: data['sender_id'] as String? ?? '',
      senderUsername: data['sender_username'] as String? ?? '',
      senderAvatarUrl: data['sender_avatar_url'] as String?,
      content: data['content'] as String? ?? '',
      type: _parseType(data['type'] as String? ?? 'text'),
      mediaUrl: data['media_url'] as String?,
      readAt: null,
      replyToId: data['reply_to_id'] as String?,
      replyToContent: data['reply_to_content'] as String?,
      reactions: {},
      createdAt: data['created_at'] != null
          ? DateTime.parse(data['created_at'] as String)
          : DateTime.now(),
    );
  }

  MessageType _parseType(String raw) {
    return MessageType.values.firstWhere(
      (t) => t.name == raw,
      orElse: () => MessageType.text,
    );
  }

  /// Optimistically prepends a message then confirms via API.
  Future<void> sendMessage(
    String content,
    MessageType type, {
    String? replyToId,
    String? replyToContent,
    String? mediaUrl,
  }) async {
    final current = state.valueOrNull ?? [];
    final currentUserId = ref.read(currentUserIdProvider);
    final tempId = 'temp_${DateTime.now().millisecondsSinceEpoch}';

    final optimistic = MessageEntity(
      id: tempId,
      conversationId: _convId,
      senderId: currentUserId,
      senderUsername: 'me',
      content: content,
      type: type,
      mediaUrl: mediaUrl,
      replyToId: replyToId,
      replyToContent: replyToContent,
      reactions: {},
      createdAt: DateTime.now(),
    );

    state = AsyncData([optimistic, ...current]);

    final result = await ref.read(sendMessageUseCaseProvider).call(
          SendMessageParams(
            conversationId: _convId,
            content: content,
            type: type,
            replyToId: replyToId,
            mediaUrl: mediaUrl,
          ),
        );

    result.fold(
      (_) {
        // Remove optimistic on failure.
        final rollback = (state.valueOrNull ?? [])
            .where((m) => m.id != tempId)
            .toList();
        state = AsyncData(rollback);
      },
      (confirmed) {
        final msgs = (state.valueOrNull ?? []).toList();
        final idx = msgs.indexWhere((m) => m.id == tempId);
        if (idx != -1) msgs[idx] = confirmed;
        state = AsyncData(msgs);
      },
    );
  }

  Future<void> loadMore() async {
    if (_isLoadingMore || _nextCursor == null) return;
    _isLoadingMore = true;
    final current = state.valueOrNull ?? [];
    final result = await ref
        .read(getMessagesUseCaseProvider)
        .call(_convId, cursor: _nextCursor);
    result.fold(
      (_) {},
      (tuple) {
        _nextCursor = tuple.$2;
        state = AsyncData([...current, ...tuple.$1]);
      },
    );
    _isLoadingMore = false;
  }

  void sendTypingEvent() {
    ref.read(webSocketServiceProvider).sendWsMessage({
      'event': 'typing',
      'conversation_id': _convId,
    });
  }

  void sendStopTypingEvent() {
    ref.read(webSocketServiceProvider).sendWsMessage({
      'event': 'stop_typing',
      'conversation_id': _convId,
    });
  }

  /// Removes a message locally (optimistic delete).
  void deleteMessage(String messageId) {
    final current = state.valueOrNull;
    if (current == null) return;
    state = AsyncData(current.where((m) => m.id != messageId).toList());
  }
}

final chatProvider =
    AsyncNotifierProviderFamily<ChatNotifier, List<MessageEntity>, String>(
  ChatNotifier.new,
);

// ---------------------------------------------------------------------------
// TypingProvider  (family by conversationId)
// ---------------------------------------------------------------------------

/// True when the other participant is typing in this conversation.
final typingProvider =
    StateProviderFamily<bool, String>((ref, convId) => false);

/// Listens to WS typing events and updates typingProvider.
/// Should be called once per chat screen via ref.listen or keepAlive.
final typingWatcherProvider = StreamProviderFamily<void, String>((ref, convId) {
  final ws = ref.watch(webSocketServiceProvider);
  return ws.messageStream
      .where(
        (e) =>
            (e['event'] == 'typing' || e['event'] == 'stop_typing') &&
            e['conversation_id'] == convId,
      )
      .map((e) {
    final isTyping = e['event'] == 'typing';
    ref.read(typingProvider(convId).notifier).state = isTyping;
  });
});
