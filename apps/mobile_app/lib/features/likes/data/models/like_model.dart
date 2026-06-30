import 'package:tiktok_clone/features/likes/domain/entities/like_entity.dart';

class LikeModel extends LikeEntity {
  const LikeModel({
    required super.videoId,
    required super.userId,
    required super.isLiked,
    required super.likeCount,
  });

  factory LikeModel.fromJson(Map<String, dynamic> j) => LikeModel(
    videoId: j['video_id'] as String,
    userId: j['user_id'] as String? ?? '',
    isLiked: j['is_liked'] as bool? ?? false,
    likeCount: (j['like_count'] as num? ?? 0).toInt(),
  );

  Map<String, dynamic> toJson() => {
    'video_id': videoId,
    'user_id': userId,
    'is_liked': isLiked,
    'like_count': likeCount,
  };
}
