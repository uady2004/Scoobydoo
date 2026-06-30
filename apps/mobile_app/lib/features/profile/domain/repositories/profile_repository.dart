import 'dart:io';

import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';

/// Domain contract for all profile-related data access.
/// Implementations live in the data layer; use-cases depend only on this.
abstract interface class ProfileRepository {
  /// Fetch public profile for [userId].
  Future<Either<Failure, ProfileEntity>> getProfile(String userId);

  /// Persist changes to the current user's profile.
  Future<Either<Failure, ProfileEntity>> updateProfile(
    Map<String, dynamic> params,
  );

  /// Replace the current user's avatar with [file].
  /// Returns the resulting CDN URL.
  Future<Either<Failure, String>> uploadAvatar(File file);

  /// Paginated videos posted by [userId].
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getUserVideos(String userId, String? cursor);

  /// Paginated videos the current user has liked.
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getLikedVideos(String? cursor);

  /// Paginated videos the current user has bookmarked.
  Future<Either<Failure, (List<FeedItemEntity>, String? nextCursor)>>
      getBookmarkedVideos(String? cursor);

  /// Follow [userId].
  Future<Either<Failure, Unit>> followUser(String userId);

  /// Unfollow [userId].
  Future<Either<Failure, Unit>> unfollowUser(String userId);
}
