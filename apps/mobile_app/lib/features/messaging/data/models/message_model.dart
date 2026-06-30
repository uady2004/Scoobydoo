import 'package:tiktok_clone/features/messaging/domain/entities/conversation_entity.dart';
import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

class MessageModel {
  final String id;
  final String conversationId;
  final String senderId;
  final String senderUsername;
  final String? senderAvatarUrl;
  final String content;
  final MessageType type;
  final String? mediaUrl;
  final DateTime? readAt;
  final String? replyToId;
  final String? replyToContent;
  final Map<String, int> reactions;
  final DateTime createdAt;

  const MessageModel({
    required this.id,
    required this.conversationId,
    required this.senderId,
    required this.senderUsername,
    this.senderAvatarUrl,
    required this.content,
    required this.type,
    this.mediaUrl,
    this.readAt,
    this.replyToId,
    this.replyToContent,
    required this.reactions,
    required this.createdAt,
  });

  factory MessageModel.fromJson(Map<String, dynamic> json) {
    return MessageModel(
      id: json['id'] as String,
      conversationId: json['conversation_id'] as String,
      senderId: json['sender_id'] as String,
      senderUsername: json['sender_username'] as String,
      senderAvatarUrl: json['sender_avatar_url'] as String?,
      content: json['content'] as String? ?? '',
      type: _parseMessageType(json['type'] as String? ?? 'text'),
      mediaUrl: json['media_url'] as String?,
      readAt: json['read_at'] != null
          ? DateTime.parse(json['read_at'] as String)
          : null,
      replyToId: json['reply_to_id'] as String?,
      replyToContent: json['reply_to_content'] as String?,
      reactions: (json['reactions'] as Map<String, dynamic>? ?? {})
          .map((k, v) => MapEntry(k, (v as num).toInt())),
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'conversation_id': conversationId,
      'sender_id': senderId,
      'sender_username': senderUsername,
      'sender_avatar_url': senderAvatarUrl,
      'content': content,
      'type': type.name,
      'media_url': mediaUrl,
      'read_at': readAt?.toIso8601String(),
      'reply_to_id': replyToId,
      'reply_to_content': replyToContent,
      'reactions': reactions,
      'created_at': createdAt.toIso8601String(),
    };
  }

  MessageEntity toEntity() {
    return MessageEntity(
      id: id,
      conversationId: conversationId,
      senderId: senderId,
      senderUsername: senderUsername,
      senderAvatarUrl: senderAvatarUrl,
      content: content,
      type: type,
      mediaUrl: mediaUrl,
      readAt: readAt,
      replyToId: replyToId,
      replyToContent: replyToContent,
      reactions: reactions,
      createdAt: createdAt,
    );
  }

  static MessageType _parseMessageType(String raw) {
    switch (raw) {
      case 'image':
        return MessageType.image;
      case 'video':
        return MessageType.video;
      case 'voice':
        return MessageType.voice;
      case 'gift':
        return MessageType.gift;
      case 'system':
        return MessageType.system;
      default:
        return MessageType.text;
    }
  }
}

class ConversationModel {
  final String id;
  final List<Map<String, dynamic>> participants;
  final MessageModel? lastMessage;
  final DateTime? lastMessageAt;
  final int unreadCount;
  final bool isGroup;
  final String? groupName;
  final String? groupAvatarUrl;

  const ConversationModel({
    required this.id,
    required this.participants,
    this.lastMessage,
    this.lastMessageAt,
    required this.unreadCount,
    required this.isGroup,
    this.groupName,
    this.groupAvatarUrl,
  });

  factory ConversationModel.fromJson(Map<String, dynamic> json) {
    return ConversationModel(
      id: json['id'] as String,
      participants: (json['participants'] as List<dynamic>? ?? [])
          .map((p) => p as Map<String, dynamic>)
          .toList(),
      lastMessage: json['last_message'] != null
          ? MessageModel.fromJson(json['last_message'] as Map<String, dynamic>)
          : null,
      lastMessageAt: json['last_message_at'] != null
          ? DateTime.parse(json['last_message_at'] as String)
          : null,
      unreadCount: (json['unread_count'] as num? ?? 0).toInt(),
      isGroup: json['is_group'] as bool? ?? false,
      groupName: json['group_name'] as String?,
      groupAvatarUrl: json['group_avatar_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'participants': participants,
      'last_message': lastMessage?.toJson(),
      'last_message_at': lastMessageAt?.toIso8601String(),
      'unread_count': unreadCount,
      'is_group': isGroup,
      'group_name': groupName,
      'group_avatar_url': groupAvatarUrl,
    };
  }

  ConversationEntity toEntity() {
    return ConversationEntity(
      id: id,
      participants: participants,
      lastMessage: lastMessage?.toEntity(),
      lastMessageAt: lastMessageAt,
      unreadCount: unreadCount,
      isGroup: isGroup,
      groupName: groupName,
      groupAvatarUrl: groupAvatarUrl,
    );
  }
}
