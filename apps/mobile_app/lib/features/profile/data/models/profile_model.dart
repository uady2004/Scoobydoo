import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';

class ProfileModel extends ProfileEntity {
  const ProfileModel({
    required super.userId,
    required super.username,
    required super.displayName,
    super.bio,
    super.avatarUrl,
    super.website,
    required super.followerCount,
    required super.followingCount,
    required super.likeCount,
    required super.videoCount,
    required super.isVerified,
    required super.isCreator,
    required super.isPrivate,
    required super.createdAt,
  });

  factory ProfileModel.fromJson(Map<String, dynamic> json) {
    // Support both 'user_id' and 'id' from different backends
    final id = json['user_id']?.toString() ??
        json['userId']?.toString() ??
        json['id']?.toString() ??
        '';
    final username = json['username']?.toString() ?? '';
    return ProfileModel(
      userId: id,
      username: username,
      displayName: json['display_name']?.toString() ??
          json['displayName']?.toString() ??
          username,
      bio: json['bio']?.toString(),
      avatarUrl: json['avatar_url']?.toString() ??
          json['avatarUrl']?.toString(),
      website: json['website']?.toString(),
      followerCount: (json['follower_count'] as num? ??
              json['followerCount'] as num? ??
              0)
          .toInt(),
      followingCount: (json['following_count'] as num? ??
              json['followingCount'] as num? ??
              0)
          .toInt(),
      likeCount: (json['like_count'] as num? ??
              json['likeCount'] as num? ??
              0)
          .toInt(),
      videoCount: (json['video_count'] as num? ??
              json['videoCount'] as num? ??
              0)
          .toInt(),
      isVerified: json['is_verified'] as bool? ??
          json['isVerified'] as bool? ??
          json['email_verified'] as bool? ??
          false,
      isCreator: json['is_creator'] as bool? ??
          json['isCreator'] as bool? ??
          false,
      isPrivate: json['is_private'] as bool? ??
          json['isPrivate'] as bool? ??
          false,
      createdAt: DateTime.tryParse(
            json['created_at']?.toString() ??
                json['createdAt']?.toString() ??
                '',
          ) ??
          DateTime.now(),
    );
  }

  /// Build a ProfileModel from auth user data (used as fallback).
  factory ProfileModel.fromAuthUser({
    required String userId,
    required String username,
    required String email,
    String? displayName,
    String? avatarUrl,
    bool isVerified = false,
  }) {
    return ProfileModel(
      userId: userId,
      username: username,
      displayName: displayName ?? username,
      bio: null,
      avatarUrl: avatarUrl,
      website: null,
      followerCount: 0,
      followingCount: 0,
      likeCount: 0,
      videoCount: 0,
      isVerified: isVerified,
      isCreator: false,
      isPrivate: false,
      createdAt: DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'user_id': userId,
      'username': username,
      'display_name': displayName,
      'bio': bio,
      'avatar_url': avatarUrl,
      'website': website,
      'follower_count': followerCount,
      'following_count': followingCount,
      'like_count': likeCount,
      'video_count': videoCount,
      'is_verified': isVerified,
      'is_creator': isCreator,
      'is_private': isPrivate,
      'created_at': createdAt.toIso8601String(),
    };
  }

  factory ProfileModel.fromEntity(ProfileEntity entity) {
    return ProfileModel(
      userId: entity.userId,
      username: entity.username,
      displayName: entity.displayName,
      bio: entity.bio,
      avatarUrl: entity.avatarUrl,
      website: entity.website,
      followerCount: entity.followerCount,
      followingCount: entity.followingCount,
      likeCount: entity.likeCount,
      videoCount: entity.videoCount,
      isVerified: entity.isVerified,
      isCreator: entity.isCreator,
      isPrivate: entity.isPrivate,
      createdAt: entity.createdAt,
    );
  }
}