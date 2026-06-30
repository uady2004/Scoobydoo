import 'dart:io';

import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

abstract class MessageRepository {
  /// Fetches paginated list of conversations.
  /// Returns (conversations, nextCursor).
  Future<Either<String, (List<ConversationEntity>, String?)>> getConversations({
    String? cursor,
  });

  /// Fetches paginated messages for a conversation.
  /// Returns (messages, nextCursor).
  Future<Either<String, (List<MessageEntity>, String?)>> getMessages(
    String conversationId, {
    String? cursor,
  });

  /// Sends a message to a conversation.
  Future<Either<String, MessageEntity>> sendMessage(
    String conversationId,
    String content,
    MessageType type, {
    String? replyToId,
    String? mediaUrl,
  });

  /// Uploads a media file and returns its remote URL.
  Future<Either<String, String>> uploadMedia(File file);

  /// Marks all messages in a conversation as read.
  Future<Either<String, Unit>> markRead(String conversationId);

  /// Creates a group conversation.
  Future<Either<String, ConversationEntity>> createGroup(
    List<String> userIds,
    String name,
  );
}
