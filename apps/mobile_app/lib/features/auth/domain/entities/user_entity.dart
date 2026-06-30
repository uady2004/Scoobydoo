import 'package:equatable/equatable.dart';

class UserEntity extends Equatable {
  final String id;
  final String username;
  final String displayName;
  final String email;
  final String? phone;
  final String? avatarUrl;
  final bool isVerified;
  final bool isCreator;
  final DateTime createdAt;

  const UserEntity({
    required this.id,
    required this.username,
    required this.displayName,
    required this.email,
    this.phone,
    this.avatarUrl,
    required this.isVerified,
    required this.isCreator,
    required this.createdAt,
  });

  UserEntity copyWith({
    String? id,
    String? username,
    String? displayName,
    String? email,
    String? phone,
    String? avatarUrl,
    bool? isVerified,
    bool? isCreator,
    DateTime? createdAt,
  }) {
    return UserEntity(
      id: id ?? this.id,
      username: username ?? this.username,
      displayName: displayName ?? this.displayName,
      email: email ?? this.email,
      phone: phone ?? this.phone,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      isVerified: isVerified ?? this.isVerified,
      isCreator: isCreator ?? this.isCreator,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
        id,
        username,
        displayName,
        email,
        phone,
        avatarUrl,
        isVerified,
        isCreator,
        createdAt,
      ];
}
