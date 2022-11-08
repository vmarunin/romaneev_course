
This is extra essigmentm tou can to if after pipeline ( signer )

You need to write worker pool that can be dynamically resized (durung prigram runtime) - you can add or delete workers&

Workers limits depends on some "load" - some external signal.

Lets imagine that you will have some "sensor" ( cup usage, ram or disk, number of processed tasks - anything - even sin/cos funcs ) and depends of this "sensor" we can say you worker pool to finish any of workers ( one of then) or can add 1 more worker in pool.

And worker pool will process tasks athat will print someting and make `time.Sleep( 500 * time.Millisecond)`

Dont make space ship! Dont over engineered

This assigmnent for using `channels`, `select` and `goroutines`.

Typical solution - 100-150-200 lines