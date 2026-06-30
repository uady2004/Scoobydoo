import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/hashtags/data/datasources/hashtag_remote_datasource.dart';
import 'package:tiktok_clone/features/hashtags/domain/entities/hashtag_entity.dart';

final hashtagDatasourceProvider = Provider<HashtagRemoteDatasource>((ref) =>
    HashtagRemoteDatasourceImpl(ApiClient.instance.dio));

final hashtagProvider = FutureProvider.family<HashtagEntity, String>((ref, tag) =>
    ref.watch(hashtagDatasourceProvider).getHashtag(tag));

final trendingHashtagsProvider = FutureProvider<List<HashtagEntity>>((ref) =>
    ref.watch(hashtagDatasourceProvider).getTrendingHashtags());
