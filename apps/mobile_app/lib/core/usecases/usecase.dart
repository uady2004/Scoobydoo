import 'package:fpdart/fpdart.dart';
import '../error/failures.dart';

/// Base contract for all use-cases that accept typed [Params].
abstract class UseCase<Result, Params> {
  Future<Either<Failure, Result>> call(Params params);
}

/// Sentinel for use-cases that take no parameters.
class NoParams {
  const NoParams();
}
