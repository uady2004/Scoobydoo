import 'package:tiktok_clone/features/comments/domain/entities/comment_entity.dart';

class CommentModel extends CommentEntity {
  const CommentModel({
    required super.id,
    required super.videoId,
    required super.userId,
    required super.username,
    required super.avatarUrl,
    required super.content,
    required super.likeCount,
    required super.isLiked,
    required super.isPinned,
    required super.replyCount,
    super.parentId,
    required super.createdAt,
  });

  factory CommentModel.fromJson(Map<String, dynamic> json) {
    return CommentModel(
      id: json['id'] as String,
      videoId: json['video_id'] as String,
      userId: json['user_id'] as String,
      username: json['username'] as String,
      avatarUrl: json['avatar_url'] as String,
      content: json['content'] as String,
      likeCount: (json['like_count'] as num?)?.toInt() ?? 0,
      isLiked: json['is_liked'] as bool? ?? false,
      isPinned: json['is_pinned'] as bool? ?? false,
      replyCount: (json['reply_count'] as num?)?.toInt() ?? 0,
      parentId: json['parent_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'video_id': videoId,
      'user_id': userId,
      'username': username,
      'avatar_url': avatarUrl,
      'content': content,
      'like_count': likeCount,
      'is_liked': isLiked,
      'is_pinned': isPinned,
      'reply_count': replyCount,
      'parent_id': parentId,
      'created_at': createdAt.toIso8601String(),
    };
  }

  factory CommentModel.fromEntity(CommentEntity entity) {
    return CommentModel(
      id: entity.id,
      videoId: entity.videoId,
      userId: entity.userId,
      username: entity.username,
      avatarUrl: entity.avatarUrl,
      content: entity.content,
      likeCount: entity.likeCount,
      isLiked: entity.isLiked,
      isPinned: entity.isPinned,
      replyCount: entity.replyCount,
      parentId: entity.parentId,
      createdAt: entity.createdAt,
    );
  }
}
