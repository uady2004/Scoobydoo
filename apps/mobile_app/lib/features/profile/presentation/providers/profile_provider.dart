import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:tiktok_clone/core/network/api_client.dart';
import 'package:tiktok_clone/features/auth/presentation/providers/auth_provider.dart';
import 'package:tiktok_clone/features/home_feed/domain/entities/feed_item_entity.dart';
import 'package:tiktok_clone/features/profile/data/datasources/profile_remote_datasource.dart';
import 'package:tiktok_clone/features/profile/data/repositories/profile_repository_impl.dart';
import 'package:tiktok_clone/features/profile/domain/entities/profile_entity.dart';
import 'package:tiktok_clone/features/profile/domain/repositories/profile_repository.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/follow_user_usecase.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/get_profile_usecase.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/get_user_videos_usecase.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/update_profile_usecase.dart';
import 'package:tiktok_clone/features/profile/domain/usecases/upload_avatar_usecase.dart';
import 'package:tiktok_clone/features/profile/data/models/profile_model.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Infrastructure
// ─────────────────────────────────────────────────────────────────────────────

final _profileDioProvider = Provider<Dio>(
  (_) => ApiClient.instance.dio,
);

final profileRemoteDatasourceProvider =
    Provider<ProfileRemoteDataSource>((ref) {
  return ProfileRemoteDataSourceImpl(ref.watch(_profileDioProvider));
});

final profileRepositoryProvider = Provider<ProfileRepository>((ref) {
  return ProfileRepositoryImpl(ref.watch(profileRemoteDatasourceProvider));
});

// ── Use-case providers ──────────────────────────────────────────────────────

final getProfileUseCaseProvider = Provider<GetProfileUseCase>((ref) {
  return GetProfileUseCase(ref.watch(profileRepositoryProvider));
});

final updateProfileUseCaseProvider = Provider<UpdateProfileUseCase>((ref) {
  return UpdateProfileUseCase(ref.watch(profileRepositoryProvider));
});

final uploadAvatarUseCaseProvider = Provider<UploadAvatarUseCase>((ref) {
  return UploadAvatarUseCase(ref.watch(profileRepositoryProvider));
});

final followUserUseCaseProvider = Provider<FollowUserUseCase>((ref) {
  return FollowUserUseCase(ref.watch(profileRepositoryProvider));
});

final getUserVideosUseCaseProvider = Provider<GetUserVideosUseCase>((ref) {
  return GetUserVideosUseCase(ref.watch(profileRepositoryProvider));
});

// ─────────────────────────────────────────────────────────────────────────────
// Profile notifier
// ─────────────────────────────────────────────────────────────────────────────

/// AsyncNotifier.family keyed by userId.
/// Fetches and caches a single user's profile.
class ProfileNotifier extends FamilyAsyncNotifier<ProfileEntity, String> {
  late final String _userId;

  @override
  Future<ProfileEntity> build(String arg) async {
    _userId = arg;
    return _fetchProfile();
  }

  Future<ProfileEntity> _fetchProfile() async {
    final useCase = ref.read(getProfileUseCaseProvider);
    final result = await useCase(GetProfileParams(userId: _userId));
    return result.fold(
      (failure) => throw Exception(failure.message),
      (profile) => profile,
    );
  }

  /// Re-fetch the profile from the network.
  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetchProfile);
  }

  /// Optimistically update the local profile after editing.
  void applyLocalUpdate(ProfileEntity updated) {
    state = AsyncValue.data(updated);
  }
}

final profileProvider =
    AsyncNotifierProviderFamily<ProfileNotifier, ProfileEntity, String>(
  ProfileNotifier.new,
);

// ─────────────────────────────────────────────────────────────────────────────
// Own profile — watches auth state to derive the current user's profile.
// ─────────────────────────────────────────────────────────────────────────────

final ownProfileProvider = FutureProvider<ProfileEntity?>((ref) async {
  final authState = await ref.watch(authProvider.future);
  if (authState is! AuthAuthenticated) return null;

  final user = authState.user;
  final result =
      await ref.read(profileRepositoryProvider).getProfile(user.id);

  return result.fold(
    // API failed — build profile directly from auth user so it always shows
    (_) => ProfileModel.fromAuthUser(
      userId: user.id,
      username: user.username,
      email: user.email,
      displayName: user.displayName,
      avatarUrl: user.avatarUrl,
      isVerified: user.isVerified,
    ),
    (profile) {
      // API succeeded but userId missing — patch it
      if (profile.userId.isEmpty) {
        return ProfileModel.fromAuthUser(
          userId: user.id,
          username: profile.username.isNotEmpty
              ? profile.username
              : user.username,
          email: user.email,
          displayName: profile.displayName,
          avatarUrl: profile.avatarUrl,
          isVerified: profile.isVerified,
        );
      }
      return profile;
    },
  );
});

// ─────────────────────────────────────────────────────────────────────────────
// Follow state
// ─────────────────────────────────────────────────────────────────────────────

/// StateProvider.family keyed by userId — true = currently following.
/// Initialised to false; callers should set the real value once the profile
/// loads (e.g. by watching profileProvider and checking an `isFollowing` flag
/// returned from the backend).
final followStateProvider =
    StateProvider.family<bool, String>((ref, userId) => false);

// ─────────────────────────────────────────────────────────────────────────────
// User videos notifier (paginated)
// ─────────────────────────────────────────────────────────────────────────────

class VideoGridState {
  const VideoGridState({
    this.items = const [],
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.nextCursor,
    this.error,
  });

  final List<FeedItemEntity> items;
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  VideoGridState copyWith({
    List<FeedItemEntity>? items,
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    String? nextCursor,
    String? error,
    bool clearError = false,
  }) {
    return VideoGridState(
      items: items ?? this.items,
      isLoading: isLoading ?? this.isLoading,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: clearError ? null : error ?? this.error,
    );
  }
}

/// Family param: a (userId, VideoTab) pair so each tab has its own notifier.
typedef UserVideosArg = ({String userId, VideoTab tab});

class UserVideosNotifier
    extends FamilyAsyncNotifier<VideoGridState, UserVideosArg> {
  late final UserVideosArg _arg;

  @override
  Future<VideoGridState> build(UserVideosArg arg) async {
    _arg = arg;
    return _loadFirstPage();
  }

  Future<VideoGridState> _loadFirstPage() async {
    final useCase = ref.read(getUserVideosUseCaseProvider);
    final result = await useCase(
      GetUserVideosParams(
        userId: _arg.userId,
        tab: _arg.tab,
      ),
    );
    return result.fold(
      (failure) => VideoGridState(error: failure.message),
      (data) {
        final (items, cursor) = data;
        return VideoGridState(
          items: items,
          nextCursor: cursor,
          hasMore: cursor != null,
        );
      },
    );
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_loadFirstPage);
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || !current.hasMore || current.isLoadingMore) return;

    state = AsyncValue.data(current.copyWith(isLoadingMore: true));

    final useCase = ref.read(getUserVideosUseCaseProvider);
    final result = await useCase(
      GetUserVideosParams(
        userId: _arg.userId,
        cursor: current.nextCursor,
        tab: _arg.tab,
      ),
    );

    result.fold(
      (failure) {
        final s = state.valueOrNull;
        if (s != null) {
          state = AsyncValue.data(
            s.copyWith(isLoadingMore: false, error: failure.message),
          );
        }
      },
      (data) {
        final (newItems, cursor) = data;
        final s = state.valueOrNull;
        if (s != null) {
          state = AsyncValue.data(
            s.copyWith(
              items: [...s.items, ...newItems],
              nextCursor: cursor,
              hasMore: cursor != null,
              isLoadingMore: false,
            ),
          );
        }
      },
    );
  }
}

final userVideosProvider =
    AsyncNotifierProviderFamily<UserVideosNotifier, VideoGridState, UserVideosArg>(
  UserVideosNotifier.new,
);

// ─────────────────────────────────────────────────────────────────────────────
// Edit profile notifier — manages the edit screen's save flow
// ─────────────────────────────────────────────────────────────────────────────

class EditProfileState {
  const EditProfileState({
    this.isSaving = false,
    this.savedSuccessfully = false,
    this.error,
  });

  final bool isSaving;
  final bool savedSuccessfully;
  final String? error;

  EditProfileState copyWith({
    bool? isSaving,
    bool? savedSuccessfully,
    String? error,
    bool clearError = false,
  }) {
    return EditProfileState(
      isSaving: isSaving ?? this.isSaving,
      savedSuccessfully: savedSuccessfully ?? this.savedSuccessfully,
      error: clearError ? null : error ?? this.error,
    );
  }
}

class EditProfileNotifier extends AsyncNotifier<EditProfileState> {
  @override
  Future<EditProfileState> build() async => const EditProfileState();

  Future<void> save({
    required UpdateProfileParams params,
    File? newAvatarFile,
    required String userId,
  }) async {
    state = AsyncValue.data(
      state.valueOrNull?.copyWith(isSaving: true, clearError: true) ??
          const EditProfileState(isSaving: true),
    );

    String? newAvatarUrl;

    // 1. Upload avatar first if one was selected.
    if (newAvatarFile != null) {
      final uploadUseCase = ref.read(uploadAvatarUseCaseProvider);
      final uploadResult =
          await uploadUseCase(UploadAvatarParams(file: newAvatarFile));
      final failed = uploadResult.fold<String?>((f) => f.message, (_) => null);
      if (failed != null) {
        state = AsyncValue.data(
          EditProfileState(isSaving: false, error: failed),
        );
        return;
      }
      newAvatarUrl = uploadResult.getOrElse((_) => '');
    }

    // 2. Persist profile text fields.
    final updateUseCase = ref.read(updateProfileUseCaseProvider);
    final result = await updateUseCase(params);

    result.fold(
      (failure) {
        state = AsyncValue.data(
          EditProfileState(isSaving: false, error: failure.message),
        );
      },
      (updatedProfile) {
        // 3. Propagate new data back into the cached profile provider.
        final patchedProfile = newAvatarUrl != null
            ? updatedProfile.copyWith(avatarUrl: newAvatarUrl)
            : updatedProfile;

        ref
            .read(profileProvider(userId).notifier)
            .applyLocalUpdate(patchedProfile);

        state = AsyncValue.data(
          const EditProfileState(isSaving: false, savedSuccessfully: true),
        );
      },
    );
  }

  void resetSaveSuccess() {
    final s = state.valueOrNull;
    if (s != null) {
      state = AsyncValue.data(s.copyWith(savedSuccessfully: false));
    }
  }
}

final editProfileProvider =
    AsyncNotifierProvider<EditProfileNotifier, EditProfileState>(
  EditProfileNotifier.new,
);
