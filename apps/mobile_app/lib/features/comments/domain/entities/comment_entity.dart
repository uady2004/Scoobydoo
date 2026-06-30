import 'package:equatable/equatable.dart';

class CommentEntity extends Equatable {
  final String id;
  final String videoId;
  final String userId;
  final String username;
  final String avatarUrl;
  final String content;
  final int likeCount;
  final bool isLiked;
  final bool isPinned;
  final int replyCount;
  final String? parentId;
  final DateTime createdAt;

  const CommentEntity({
    required this.id,
    required this.videoId,
    required this.userId,
    required this.username,
    required this.avatarUrl,
    required this.content,
    required this.likeCount,
    required this.isLiked,
    required this.isPinned,
    required this.replyCount,
    this.parentId,
    required this.createdAt,
  });

  CommentEntity copyWith({
    String? id,
    String? videoId,
    String? userId,
    String? username,
    String? avatarUrl,
    String? content,
    int? likeCount,
    bool? isLiked,
    bool? isPinned,
    int? replyCount,
    String? parentId,
    DateTime? createdAt,
  }) {
    return CommentEntity(
      id: id ?? this.id,
      videoId: videoId ?? this.videoId,
      userId: userId ?? this.userId,
      username: username ?? this.username,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      content: content ?? this.content,
      likeCount: likeCount ?? this.likeCount,
      isLiked: isLiked ?? this.isLiked,
      isPinned: isPinned ?? this.isPinned,
      replyCount: replyCount ?? this.replyCount,
      parentId: parentId ?? this.parentId,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
        id,
        videoId,
        userId,
        username,
        avatarUrl,
        content,
        likeCount,
        isLiked,
        isPinned,
        replyCount,
        parentId,
        createdAt,
      ];
}
