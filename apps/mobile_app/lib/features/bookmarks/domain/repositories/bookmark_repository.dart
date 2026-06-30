import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/bookmark_entity.dart';

abstract interface class BookmarkRepository {
  Future<Either<Failure, bool>> toggleBookmark(String videoId);
  Future<Either<Failure, List<BookmarkEntity>>> getBookmarks({String? cursor});
  Future<Either<Failure, bool>> isBookmarked(String videoId);
}
