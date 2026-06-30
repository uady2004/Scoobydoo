import 'package:tiktok_clone/features/bookmarks/domain/entities/bookmark_entity.dart';

class BookmarkModel extends BookmarkEntity {
  const BookmarkModel({
    required super.id,
    required super.videoId,
    required super.userId,
    required super.createdAt,
  });

  factory BookmarkModel.fromJson(Map<String, dynamic> j) => BookmarkModel(
    id: j['id'] as String,
    videoId: j['video_id'] as String,
    userId: j['user_id'] as String? ?? '',
    createdAt: DateTime.parse(j['created_at'] as String),
  );
}
