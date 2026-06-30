import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../repositories/bookmark_repository.dart';

class ToggleBookmarkUsecase {
  const ToggleBookmarkUsecase(this._repo);
  final BookmarkRepository _repo;
  Future<Either<Failure, bool>> call(String videoId) => _repo.toggleBookmark(videoId);
}
