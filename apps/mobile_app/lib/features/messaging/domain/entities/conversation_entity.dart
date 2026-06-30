import 'package:tiktok_clone/features/messaging/domain/entities/message_entity.dart';

class ConversationEntity {
  final String id;
  final List<Map<String, dynamic>> participants;
  final MessageEntity? lastMessage;
  final DateTime? lastMessageAt;
  final int unreadCount;
  final bool isGroup;
  final String? groupName;
  final String? groupAvatarUrl;

  const ConversationEntity({
    required this.id,
    required this.participants,
    this.lastMessage,
    this.lastMessageAt,
    required this.unreadCount,
    required this.isGroup,
    this.groupName,
    this.groupAvatarUrl,
  });

  /// Returns a display name for the conversation.
  /// For groups uses groupName; for DMs uses the other participant's username.
  String displayName(String currentUserId) {
    if (isGroup) return groupName ?? 'Group';
    final other = participants.firstWhere(
      (p) => p['id'] != currentUserId,
      orElse: () => participants.isNotEmpty ? participants.first : {},
    );
    return other['username'] as String? ?? 'Unknown';
  }

  /// Returns avatar URL for the conversation.
  String? avatarUrl(String currentUserId) {
    if (isGroup) return groupAvatarUrl;
    final other = participants.firstWhere(
      (p) => p['id'] != currentUserId,
      orElse: () => participants.isNotEmpty ? participants.first : {},
    );
    return other['avatar_url'] as String?;
  }

  /// Whether the other participant is online (for DMs).
  bool isOnline(String currentUserId) {
    if (isGroup) return false;
    final other = participants.firstWhere(
      (p) => p['id'] != currentUserId,
      orElse: () => <String, dynamic>{},
    );
    return other['is_online'] as bool? ?? false;
  }

  ConversationEntity copyWith({
    String? id,
    List<Map<String, dynamic>>? participants,
    MessageEntity? lastMessage,
    DateTime? lastMessageAt,
    int? unreadCount,
    bool? isGroup,
    String? groupName,
    String? groupAvatarUrl,
  }) {
    return ConversationEntity(
      id: id ?? this.id,
      participants: participants ?? this.participants,
      lastMessage: lastMessage ?? this.lastMessage,
      lastMessageAt: lastMessageAt ?? this.lastMessageAt,
      unreadCount: unreadCount ?? this.unreadCount,
      isGroup: isGroup ?? this.isGroup,
      groupName: groupName ?? this.groupName,
      groupAvatarUrl: groupAvatarUrl ?? this.groupAvatarUrl,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ConversationEntity &&
          runtimeType == other.runtimeType &&
          id == other.id;

  @override
  int get hashCode => id.hashCode;
}
