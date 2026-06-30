import '../../domain/entities/user_entity.dart';

class UserModel extends UserEntity {
  const UserModel({
    required super.id,
    required super.username,
    required super.displayName,
    required super.email,
    super.phone,
    super.avatarUrl,
    required super.isVerified,
    required super.isCreator,
    required super.createdAt,
  });

  factory UserModel.fromJson(Map<String, dynamic> json) {
    final username = (json['username'] ?? '').toString();
    return UserModel(
      id: (json['id'] ?? json['user_id'] ?? json['uid'] ?? '').toString(),
      username: username,
      displayName: (json['display_name'] ??
              json['displayName'] ??
              username)
          .toString(),
      email: (json['email'] ?? '').toString(),
      phone: json['phone']?.toString(),
      avatarUrl: (json['avatar_url'] ?? json['avatarUrl'])?.toString(),
      isVerified: (json['is_verified'] ??
              json['email_verified'] ??
              json['isVerified'] ??
              false) as bool,
      isCreator:
          (json['is_creator'] ?? json['isCreator'] ?? false) as bool,
      createdAt: DateTime.tryParse(
              (json['created_at'] ?? json['createdAt'] ?? '').toString()) ??
          DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'username': username,
        'display_name': displayName,
        'email': email,
        'phone': phone,
        'avatar_url': avatarUrl,
        'is_verified': isVerified,
        'is_creator': isCreator,
        'created_at': createdAt.toIso8601String(),
      };

  factory UserModel.fromEntity(UserEntity e) => UserModel(
        id: e.id,
        username: e.username,
        displayName: e.displayName,
        email: e.email,
        phone: e.phone,
        avatarUrl: e.avatarUrl,
        isVerified: e.isVerified,
        isCreator: e.isCreator,
        createdAt: e.createdAt,
      );
}