import '../../../hashtags/domain/entities/hashtag_entity.dart';

class HashtagModel extends HashtagEntity {
  const HashtagModel({
    required super.tag,
    required super.videoCount,
    required super.viewCount,
    super.coverUrl,
    super.isTrending,
    super.description,
  });

  factory HashtagModel.fromJson(Map<String, dynamic> j) => HashtagModel(
    tag: j['tag'] as String,
    videoCount: (j['video_count'] as num? ?? 0).toInt(),
    viewCount: (j['view_count'] as num? ?? 0).toInt(),
    coverUrl: j['cover_url'] as String?,
    isTrending: j['is_trending'] as bool? ?? false,
    description: j['description'] as String?,
  );
}
