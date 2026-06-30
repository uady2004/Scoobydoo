import 'package:equatable/equatable.dart';

/// Immutable domain entity representing a user's public profile.
/// This is the single source of truth across the profile feature.
class ProfileEntity extends Equatable {
  final String userId;
  final String username;
  final String displayName;
  final String? bio;
  final String? avatarUrl;
  final String? website;
  final int followerCount;
  final int followingCount;
  final int likeCount;
  final int videoCount;
  final bool isVerified;
  final bool isCreator;
  final bool isPrivate;
  final DateTime createdAt;

  const ProfileEntity({
    required this.userId,
    required this.username,
    required this.displayName,
    this.bio,
    this.avatarUrl,
    this.website,
    required this.followerCount,
    required this.followingCount,
    required this.likeCount,
    required this.videoCount,
    required this.isVerified,
    required this.isCreator,
    required this.isPrivate,
    required this.createdAt,
  });

  ProfileEntity copyWith({
    String? userId,
    String? username,
    String? displayName,
    String? bio,
    String? avatarUrl,
    String? website,
    int? followerCount,
    int? followingCount,
    int? likeCount,
    int? videoCount,
    bool? isVerified,
    bool? isCreator,
    bool? isPrivate,
    DateTime? createdAt,
    bool clearBio = false,
    bool clearAvatar = false,
    bool clearWebsite = false,
  }) {
    return ProfileEntity(
      userId: userId ?? this.userId,
      username: username ?? this.username,
      displayName: displayName ?? this.displayName,
      bio: clearBio ? null : bio ?? this.bio,
      avatarUrl: clearAvatar ? null : avatarUrl ?? this.avatarUrl,
      website: clearWebsite ? null : website ?? this.website,
      followerCount: followerCount ?? this.followerCount,
      followingCount: followingCount ?? this.followingCount,
      likeCount: likeCount ?? this.likeCount,
      videoCount: videoCount ?? this.videoCount,
      isVerified: isVerified ?? this.isVerified,
      isCreator: isCreator ?? this.isCreator,
      isPrivate: isPrivate ?? this.isPrivate,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
        userId,
        username,
        displayName,
        bio,
        avatarUrl,
        website,
        followerCount,
        followingCount,
        likeCount,
        videoCount,
        isVerified,
        isCreator,
        isPrivate,
        createdAt,
      ];
}
