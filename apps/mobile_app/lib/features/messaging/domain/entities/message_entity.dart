enum MessageType { text, image, video, voice, gift, system }

class MessageEntity {
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

  const MessageEntity({
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

  MessageEntity copyWith({
    String? id,
    String? conversationId,
    String? senderId,
    String? senderUsername,
    String? senderAvatarUrl,
    String? content,
    MessageType? type,
    String? mediaUrl,
    DateTime? readAt,
    String? replyToId,
    String? replyToContent,
    Map<String, int>? reactions,
    DateTime? createdAt,
  }) {
    return MessageEntity(
      id: id ?? this.id,
      conversationId: conversationId ?? this.conversationId,
      senderId: senderId ?? this.senderId,
      senderUsername: senderUsername ?? this.senderUsername,
      senderAvatarUrl: senderAvatarUrl ?? this.senderAvatarUrl,
      content: content ?? this.content,
      type: type ?? this.type,
      mediaUrl: mediaUrl ?? this.mediaUrl,
      readAt: readAt ?? this.readAt,
      replyToId: replyToId ?? this.replyToId,
      replyToContent: replyToContent ?? this.replyToContent,
      reactions: reactions ?? this.reactions,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is MessageEntity &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;
}
